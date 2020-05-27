package telegram

import (
	"context"
	"errors"
	"fmt"
	"github.com/robfig/cron/v3"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/hako/durafmt"
	"github.com/metalmatze/alertmanager-bot/pkg/alertmanager"
	"github.com/oklog/run"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/tucnak/telebot.v2"
)

const (
	commandStart = "/start"
	commandStop  = "/stop"
	commandHelp  = "/help"
	commandChats = "/chats"

	commandStatus     	= "/status"
	commandAlerts     	= "/alerts"
	commandSilences   	= "/silences"
	commandMute 	  	= "/mute"
	commandMuteDel    	= "/mute_del"
	commandEnvironments	= "/environments"
	commandProjects 	= "/projects"
	commandMutedEnvs	= "/muted_envs"
	commandMutedPrs		= "/muted_prs"
	commandSilenceAdd 	= "/silence_add"
	commandSilence    	= "/silence"
	commandSilenceDel 	= "/silence_del"

	responseStart = "Hey, %s! I will now keep you up to date!\n" + commandHelp
	responseStop  = "Alright, %s! I won't talk to you again.\n" + commandHelp
	responseHelp  = `
I'm a Prometheus AlertManager Bot for Telegram. I will notify you about alerts.
You can also ask me about my ` + commandStatus + `, ` + commandAlerts + ` & ` + commandSilences + `

Available commands:
` + commandStart + ` - Subscribe for alerts.
` + commandStop + ` - Unsubscribe for alerts.
` + commandStatus + ` - Print the current status.
` + commandAlerts + ` - List all alerts.
` + commandSilences + ` - List all silences.
` + commandChats + ` - List all users and group chats that subscribed.
` + commandMute + ` - Mute environments and/or projects.
` + commandMuteDel + ` - Delete mute.
` + commandEnvironments + ` - List all environments for alerts.
` + commandProjects + ` - List all projects for alerts.
` + commandMutedEnvs + ` - List all muted environments.
` + commandMutedPrs + ` - List all muted projects.
`
	ProjectAndEnvironmentMuteRegexp  = `/mute environment\[(\w+(\s*,\s*\w+)*)\],[ ]?project\[(\w+(\s*,\s*\w+)*)\]`
	MuteProjectRegexp = `/mute project\[(\w+(\s*,\s*\w+)*)\]`
	MuteEnvironmentRegexp = `/mute environment\[(\w+(\s*,\s*\w+)*)\]`
	ProjectAndEnvironmentUnmuteRegexp  = `/mute_del environment\[(\w+(\s*,\s*\w+)*)\],[ ]?project\[(\w+(\s*,\s*\w+)*)\]`
	UnmuteProjectRegexp = `/mute_del project\[(\w+(\s*,\s*\w+)*)\]`
	UnmuteEnvironmentRegexp = `/mute_del environment\[(\w+(\s*,\s*\w+)*)\]`
	EnvironmentValuesRegexp = `environment\[(.*?)\]`
	ProjectValuesRegexp = `project\[(.*?)\]`
)

// BotChatStore is all the Bot needs to store and read
type BotChatStore interface {
	List() ([]ChatInfo, error)
	AddChat(*telebot.Chat, []string, []string) error
	GetChatInfo(*telebot.Chat) (ChatInfo, error)
	RemoveChat(*telebot.Chat) error
	MuteEnvironments(*telebot.Chat, []string, []string) error
	MuteProjects(*telebot.Chat, []string, []string) error
	UnmuteEnvironment(*telebot.Chat, string, []string) error
	UnmuteProject(*telebot.Chat, string, []string) error
	MutedEnvironments(*telebot.Chat) ([]string, error)
	MutedProjects(*telebot.Chat) ([]string, error)
	AddMessage(*telebot.Message) error
	GetAllMessages() ([]telebot.Message, error)
	GetMessagesForPeriodInMinutes(float64) ([]telebot.Message, error)
	DeleteAllMessages() error
}

// Bot runs the alertmanager telegram
type Bot struct {
	addr         			string
	admins       			[]int // must be kept sorted
	environments			[]string
	projects				[]string
	environmentsAndOther 	[]string
	projectsAndOther		[]string
	fetchPeriod				float64
	deletePeriod			float64
	alertmanager 			*url.URL
	templates    			*template.Template
	chats        			BotChatStore
	logger       			log.Logger
	revision     			string
	startTime    			time.Time

	telegram *telebot.Bot

	commandsCounter *prometheus.CounterVec
	webhooksCounter prometheus.Counter
}

// BotOption passed to NewBot to change the default instance
type BotOption func(b *Bot)

// NewBot creates a Bot with the UserStore and telegram telegram
func NewBot(chats BotChatStore, token string, admin int, opts ...BotOption) (*Bot, error) {
	bot, err := telebot.NewBot(telebot.Settings{
		Token:	token,
		Poller:	&telebot.LongPoller{Timeout: 10 * time.Second},
	})
	
	if err != nil {
		return nil, err
	}

	commandsCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "alertmanagerbot",
		Name:      "commands_total",
		Help:      "Number of commands received by command name",
	}, []string{"command"})
	if err := prometheus.Register(commandsCounter); err != nil {
		return nil, err
	}

	b := &Bot{
		logger:          log.NewNopLogger(),
		telegram:        bot,
		chats:           chats,
		addr:            "127.0.0.1:8080",
		admins:          []int{admin},
		alertmanager:    &url.URL{Host: "localhost:9093"},
		commandsCounter: commandsCounter,
		// TODO: initialize templates with default?
	}

	for _, opt := range opts {
		opt(b)
	}

	return b, nil
}

// WithLogger sets the logger for the Bot as an option
func WithLogger(l log.Logger) BotOption {
	return func(b *Bot) {
		b.logger = l
	}
}

// WithAddr sets the internal listening addr of the bot's web server receiving webhooks
func WithAddr(addr string) BotOption {
	return func(b *Bot) {
		b.addr = addr
	}
}

// WithAlertmanager sets the connection url for the Alertmanager
func WithAlertmanager(u *url.URL) BotOption {
	return func(b *Bot) {
		b.alertmanager = u
	}
}

// WithTemplates uses Alertmanager template to render messages for Telegram
func WithTemplates(t *template.Template) BotOption {
	return func(b *Bot) {
		b.templates = t
	}
}

// WithRevision is setting the Bot's revision for status commands
func WithRevision(r string) BotOption {
	return func(b *Bot) {
		b.revision = r
	}
}

// WithStartTime is setting the Bot's start time for status commands
func WithStartTime(st time.Time) BotOption {
	return func(b *Bot) {
		b.startTime = st
	}
}

// WithExtraAdmins allows the specified additional user IDs to issue admin
// commands to the bot.
func WithExtraAdmins(ids ...int) BotOption {
	return func(b *Bot) {
		b.admins = append(b.admins, ids...)
		sort.Ints(b.admins)
	}
}

// WithEnvironments allows to define environments that are monitored by Prometheus
func WithEnvironments(environmentsToUse string) BotOption {
	return func(b *Bot) {
		p := strings.Replace(environmentsToUse, " ", "", -1)
		environmentsToSave := strings.Split(p, ",")
		b.environments = append(b.environments, environmentsToSave...)
		b.environmentsAndOther = append(b.environments, "other")
	}
}

// WithProjects allows to define projects that are monitored by Prometheus
func WithProjects(projectsToUse string) BotOption {
	return func(b *Bot) {
		p := strings.Replace(projectsToUse, " ", "", -1)
		projectsToSave := strings.Split(p, ",")
		b.projects = append(b.projects, projectsToSave...)
		b.projectsAndOther = append(b.projects, "other")
	}
}

// WithFetchPeriod allows to define scheduler period for fetching messages from store
func WithFetchPeriod(fetchPeriod float64) BotOption {
	return func(b *Bot) {
		b.fetchPeriod = fetchPeriod
	}
}

// WithDeletePeriod allows to define period of deleting messages
func WithDeletePeriod(deletePeriod float64) BotOption {
	return func(b *Bot) {
		b.deletePeriod = deletePeriod
	}
}

// SendAdminMessage to the admin's ID with a message
func (b *Bot) SendAdminMessage(adminID int, message string) {
	b.telegram.Send(&telebot.User{ID: adminID}, message)
}

// isAdminID returns whether id is one of the configured admin IDs.
func (b *Bot) isAdminID(id int) bool {
	i := sort.SearchInts(b.admins, id)
	return i < len(b.admins) && b.admins[i] == id
}

// Run the telegram and listen to messages send to the telegram
func (b *Bot) Run(ctx context.Context, webhooks <-chan notify.WebhookMessage) error {
	var gr run.Group
	{
		gr.Add(func() error {
			return b.sendWebhook(ctx, webhooks)
		}, func(err error) {
		})
	}
	{
		gr.Add(func() error {
			scheduler := cron.New(cron.WithLocation(time.UTC))
			scheduler.AddFunc(fmt.Sprintf("@every %fm", b.fetchPeriod), func() {
				messages, err := b.chats.GetMessagesForPeriodInMinutes(b.deletePeriod)
				if err != nil {
					level.Warn(b.logger).Log("msg", "cannot find messages to delete", err)
				}

				for _, msg := range messages {
					err = b.telegram.Delete(&msg)
					if err != nil {
						level.Warn(b.logger).Log("msg", "cannot delete message", err)
					}
				}
			})
			scheduler.Start()
			return nil
		}, func(err error) {
		})
	}
	{
		gr.Add(func() error {
			b.telegram.Handle(commandStart, b.handleStart)
			b.telegram.Handle(commandStop, b.handleStop)
			b.telegram.Handle(commandHelp, b.handleHelp)
			b.telegram.Handle(commandChats, b.handleChats)
			b.telegram.Handle(commandStatus, b.handleStatus)
			b.telegram.Handle(commandAlerts, b.handleAlerts)
			b.telegram.Handle(commandSilences, b.handleSilences)
			b.telegram.Handle(commandMute, b.handleMute)
			b.telegram.Handle(commandMuteDel, b.handleMuteDel)
			b.telegram.Handle(commandEnvironments, b.handleEnvironments)
			b.telegram.Handle(commandProjects, b.handleProjects)
			b.telegram.Handle(commandMutedEnvs, b.handleMutedEnvs)
			b.telegram.Handle(commandMutedPrs, b.handleMutedPrs)
			b.telegram.Start()
			return nil
		}, func(err error) {
		})
	}
	return gr.Run()
}

func (b *Bot) checkMessage(message *telebot.Message) error {
	level.Debug(b.logger).Log("msg", "message received", "text", message.Text)
	if message.IsService() {
		return nil
	}
	if !b.isAdminID(message.Sender.ID) {
		b.commandsCounter.WithLabelValues("dropped").Inc()
		return fmt.Errorf("dropped message from forbidden sender")
	}

	if err := b.telegram.Notify(message.Chat, telebot.Typing); err != nil {
		return err
	}
	return nil
}

// sendWebhook sends messages received via webhook to all subscribed chats
func (b *Bot) sendWebhook(ctx context.Context, webhooks <-chan notify.WebhookMessage) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case w := <-webhooks:
			receiversAndMessages := make(map[telebot.Chat]template.Data)
			for _, alert := range w.Alerts {
				alertEnvironmentName := alert.Labels["environment"]
				if !contains(b.environments, alertEnvironmentName) {
					alertEnvironmentName = "other"
				}

				alertProjectName := alert.Labels["project"]
				if !contains(b.projects, alertProjectName) {
					alertProjectName = "other"
				}

				chatInfos, err := b.chats.List()
				if err != nil {
					level.Error(b.logger).Log("msg", "failed to get chat list from store", "err", err)
					continue
				}

				for _, chatInfo := range chatInfos {
					alertEnvs := chatInfo.AlertEnvironments
					alertPrs := chatInfo.AlertProjects
					if contains(alertEnvs, alertEnvironmentName) && contains(alertPrs, alertProjectName) {
						data := &template.Data{
							Receiver:          w.Receiver,
							Status:            w.Status,
							Alerts:            []template.Alert{alert},
							GroupLabels:       w.GroupLabels,
							CommonLabels:      w.CommonLabels,
							CommonAnnotations: w.CommonAnnotations,
							ExternalURL:       w.ExternalURL,
						}

						if _, exists := receiversAndMessages[*chatInfo.Chat]; exists {
							data.Alerts = append(data.Alerts, receiversAndMessages[*chatInfo.Chat].Alerts...)
							receiversAndMessages[*chatInfo.Chat] = *data
						} else {
							receiversAndMessages[*chatInfo.Chat] = *data
						}
					}
				}
			}

			for k, v := range receiversAndMessages {
				out, err := b.templates.ExecuteHTMLString(`{{ template "telegram.default" . }}`, v)
				if err != nil {
					level.Warn(b.logger).Log("msg", "failed to template alerts", "err", err)
					continue
				}
				msg, err := b.telegram.Send(&telebot.Chat{ID: k.ID}, b.truncateMessage(out), &telebot.SendOptions{ParseMode: telebot.ModeHTML})
				if err != nil {
					level.Warn(b.logger).Log("msg", "failed to send message to subscribed chat", "err", err)
				}
				err = b.chats.AddMessage(msg)
				if err != nil {
					level.Warn(b.logger).Log("msg", "failed to save response message to store", err)
				}
			}
		}
	}
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if 0 == strings.Compare(v, value) {
			return true
		}
	}
	return false
}

func (b *Bot) handleStart(message *telebot.Message) {
	if err := b.checkMessage(message); err != nil {
		level.Info(b.logger).Log(
			"msg", "failed to process message",
			"err", err,
			"sender_id", message.Sender.ID,
			"sender_username", message.Sender.Username,
		)
	} else {
		if err := b.chats.AddChat(message.Chat, b.environmentsAndOther, b.projectsAndOther); err != nil {
			level.Warn(b.logger).Log("msg", "failed to add chat to chat store", "err", err)
			b.telegram.Send(message.Chat, "I can't add this chat to the subscribers list.")
			return
		}

		b.telegram.Send(message.Chat, fmt.Sprintf(responseStart, message.Sender.FirstName))
		level.Info(b.logger).Log(
			"user subscribed",
			"username", message.Sender.Username,
			"user_id", message.Sender.ID,
		)
	}
}

func (b *Bot) handleStop(message *telebot.Message) {
	if err := b.checkMessage(message); err != nil {
		level.Info(b.logger).Log(
			"msg", "failed to process message",
			"err", err,
			"sender_id", message.Sender.ID,
			"sender_username", message.Sender.Username,
		)
	} else {
		if err := b.chats.RemoveChat(message.Chat); err != nil {
			level.Warn(b.logger).Log("msg", "failed to remove chat from chat store", "err", err)
			b.telegram.Send(message.Chat, "I can't remove this chat from the subscribers list.")
			return
		}

		b.telegram.Send(message.Chat, fmt.Sprintf(responseStop, message.Sender.FirstName))
		level.Info(b.logger).Log(
			"user unsubscribed",
			"username", message.Sender.Username,
			"user_id", message.Sender.ID,
		)
	}
}

func (b *Bot) handleHelp(message *telebot.Message) {
	if err := b.checkMessage(message); err != nil {
		level.Info(b.logger).Log(
			"msg", "failed to process message",
			"err", err,
			"sender_id", message.Sender.ID,
			"sender_username", message.Sender.Username,
		)
	} else {
		b.telegram.Send(message.Chat, responseHelp)
	}
}

func (b *Bot) handleChats(message *telebot.Message) {
	if err := b.checkMessage(message); err != nil {
		level.Info(b.logger).Log(
			"msg", "failed to process message",
			"err", err,
			"sender_id", message.Sender.ID,
			"sender_username", message.Sender.Username,
		)
	} else {
		chats, err := b.chats.List()
		if err != nil {
			level.Warn(b.logger).Log("msg", "failed to list chats from chat store", "err", err)
			b.telegram.Send(message.Chat, "I can't list the subscribed chats.")
			return
		}

		list := ""
		for _, chat := range chats {
			if chat.Chat.Type == telebot.ChatGroup {
				list = list + fmt.Sprintf("@%s\n", chat.Chat.Title)
			} else {
				list = list + fmt.Sprintf("@%s\n", chat.Chat.Username)
			}
		}

		b.telegram.Send(message.Chat, "Currently these chat have subscribed:\n"+list)
	}
}

func (b *Bot) handleStatus(message *telebot.Message) {
	if err := b.checkMessage(message); err != nil {
		level.Info(b.logger).Log(
			"msg", "failed to process message",
			"err", err,
			"sender_id", message.Sender.ID,
			"sender_username", message.Sender.Username,
		)
	} else {
		s, err := alertmanager.Status(b.logger, b.alertmanager.String())
		if err != nil {
			level.Warn(b.logger).Log("msg", "failed to get status", "err", err)
			b.telegram.Send(message.Chat, fmt.Sprintf("failed to get status... %v", err))
			return
		}

		uptime := durafmt.Parse(time.Since(s.Data.Uptime))
		uptimeBot := durafmt.Parse(time.Since(b.startTime))

		b.telegram.Send(
			message.Chat,
			fmt.Sprintf(
				"*AlertManager*\nVersion: %s\nUptime: %s\n*AlertManager Bot*\nVersion: %s\nUptime: %s",
				s.Data.VersionInfo.Version,
				uptime,
				b.revision,
				uptimeBot,
			),
			&telebot.SendOptions{ParseMode: telebot.ModeMarkdown},
		)
	}
}

func (b *Bot) handleAlerts(message *telebot.Message) {
	if err := b.checkMessage(message); err != nil {
		level.Info(b.logger).Log(
			"msg", "failed to process message",
			"err", err,
			"sender_id", message.Sender.ID,
			"sender_username", message.Sender.Username,
		)
	} else {
		alerts, err := alertmanager.ListAlerts(b.logger, b.alertmanager.String())
		if err != nil {
			b.telegram.Send(message.Chat, fmt.Sprintf("failed to list alerts... %v", err))
			return
		}

		if len(alerts) == 0 {
			b.telegram.Send(message.Chat, "No alerts right now! 🎉")
			return
		}

		out, err := b.tmplAlerts(alerts...)
		if err != nil {
			return
		}

		_, err = b.telegram.Send(message.Chat, b.truncateMessage(out), &telebot.SendOptions{
			ParseMode: telebot.ModeHTML,
		})
		if err != nil {
			level.Warn(b.logger).Log("msg", "failed to send message", "err", err)
		}
	}
}

func (b *Bot) handleSilences(message *telebot.Message) {
	if err := b.checkMessage(message); err != nil {
		level.Info(b.logger).Log(
			"msg", "failed to process message",
			"err", err,
			"sender_id", message.Sender.ID,
			"sender_username", message.Sender.Username,
		)
	} else {
		silences, err := alertmanager.ListSilences(b.logger, b.alertmanager.String())
		if err != nil {
			b.telegram.Send(message.Chat, fmt.Sprintf("failed to list silences... %v", err))
			return
		}

		if len(silences) == 0 {
			b.telegram.Send(message.Chat, "No silences right now.")
			return
		}

		var out string
		for _, silence := range silences {
			out = out + alertmanager.SilenceMessage(silence) + "\n"
		}

		b.telegram.Send(message.Chat, out, &telebot.SendOptions{ParseMode: telebot.ModeMarkdown})
	}
}

func (b *Bot) handleMute(message *telebot.Message) {
	if err := b.checkMessage(message); err != nil {
		level.Info(b.logger).Log(
			"msg", "failed to process message",
			"err", err,
			"sender_id", message.Sender.ID,
			"sender_username", message.Sender.Username,
		)
	} else {
		envsToMute, prsToMute, err := parseMuteCommand(message.Text)
		if err != nil {
			b.telegram.Send(message.Chat, fmt.Sprintf("failed to parse mute command... %v", err))
			return
		}

		if len(envsToMute) > 0 {
			err := b.chats.MuteEnvironments(message.Chat, envsToMute, b.environmentsAndOther)
			if err != nil {
				level.Warn(b.logger).Log("msg", "failed to subscribe user to environments", "err", err)
				b.telegram.Send(message.Chat, fmt.Sprintf("failed to subscribe user to environments... %v", err))
			}
		}

		if len(prsToMute) > 0 {
			err := b.chats.MuteProjects(message.Chat, prsToMute, b.projectsAndOther)
			if err != nil {
				level.Warn(b.logger).Log("msg", "failed to subscribe user to project", "err", err)
				b.telegram.Send(message.Chat, fmt.Sprintf("failed to subscribe user to proj... %v", err))
			}
		}

		b.telegram.Send(message.Chat, "You were successfully muted environments and/or projects")
	}
}

func (b *Bot) handleMuteDel(message *telebot.Message) {
	if err := b.checkMessage(message); err != nil {
		level.Info(b.logger).Log(
			"msg", "failed to process message",
			"err", err,
			"sender_id", message.Sender.ID,
			"sender_username", message.Sender.Username,
		)
	} else {
		envsToUnmute, prsToUnmute, err := parseUnmuteCommand(message.Text)
		if err != nil {
			b.telegram.Send(message.Chat, fmt.Sprintf("failed to parse unmute command... %v", err))
			return
		}

		if len(envsToUnmute) > 0 {
			for _, env := range envsToUnmute {
				err := b.chats.UnmuteEnvironment(message.Chat, env, b.environmentsAndOther)
				if err != nil {
					level.Warn(b.logger).Log("msg", "failed to unsubscribe user from an environment", "err", err)
					b.telegram.Send(message.Chat, fmt.Sprintf("failed to unsubscribe user from an environment... %v", err))
				}
			}
		}

		if len(prsToUnmute) > 0 {
			for _, pr := range prsToUnmute {
				err := b.chats.UnmuteProject(message.Chat, pr, b.projectsAndOther)
				if err != nil {
					level.Warn(b.logger).Log("msg", "failed to unsubscribe user from a project", "err", err)
					b.telegram.Send(message.Chat, fmt.Sprintf("failed to unsubscribe user from a project... %v", err))
				}
			}
		}

		b.telegram.Send(message.Chat, "You were successfully delete mute from environments and/or projects")
	}
}

func (b *Bot) handleEnvironments(message *telebot.Message) {
	if err := b.checkMessage(message); err != nil {
		level.Info(b.logger).Log(
			"msg", "failed to process message",
			"err", err,
			"sender_id", message.Sender.ID,
			"sender_username", message.Sender.Username,
		)
	} else {
		b.telegram.Send(message.Chat, fmt.Sprintf("The following environments are available: %s", b.environmentsAndOther))
	}
}

func (b *Bot) handleProjects(message *telebot.Message) {
	if err := b.checkMessage(message); err != nil {
		level.Info(b.logger).Log(
			"msg", "failed to process message",
			"err", err,
			"sender_id", message.Sender.ID,
			"sender_username", message.Sender.Username,
		)
	} else {
		b.telegram.Send(message.Chat, fmt.Sprintf("The following projects are available: %s", b.projectsAndOther))
	}
}

func (b *Bot) handleMutedEnvs(message *telebot.Message) {
	if err := b.checkMessage(message); err != nil {
		level.Info(b.logger).Log(
			"msg", "failed to process message",
			"err", err,
			"sender_id", message.Sender.ID,
			"sender_username", message.Sender.Username,
		)
	} else {
		mutedEnvs, err := b.chats.MutedEnvironments(message.Chat)
		if err != nil {
			level.Warn(b.logger).Log("msg", "failed to get muted environments", "err", err)
			b.telegram.Send(message.Chat, fmt.Sprintf("failed to get muted environments... %v", err))
		}
		if len(mutedEnvs) > 0 {
			b.telegram.Send(message.Chat, fmt.Sprintf("Muted environments:  %s", mutedEnvs))
		} else {
			b.telegram.Send(message.Chat, "No muted environments")
		}
	}
}

func (b *Bot) handleMutedPrs(message *telebot.Message) {
	if err := b.checkMessage(message); err != nil {
		level.Info(b.logger).Log(
			"msg", "failed to process message",
			"err", err,
			"sender_id", message.Sender.ID,
			"sender_username", message.Sender.Username,
		)
	} else {
		mutedPrs, err := b.chats.MutedProjects(message.Chat)
		if err != nil {
			level.Warn(b.logger).Log("msg", "failed to get muted projects", "err", err)
			b.telegram.Send(message.Chat, fmt.Sprintf("failed to get muted projects... %v", err))
		}
		if len(mutedPrs) > 0 {
			b.telegram.Send(message.Chat, fmt.Sprintf("Muted projects:  %s", mutedPrs))
		} else {
			b.telegram.Send(message.Chat, "No muted projects")
		}
	}
}

func (b *Bot) tmplAlerts(alerts ...*types.Alert) (string, error) {
	data := b.templates.Data("default", nil, alerts...)

	out, err := b.templates.ExecuteHTMLString(`{{ template "telegram.default" . }}`, data)
	if err != nil {
		return "", err
	}

	return out, nil
}

// Truncate very big message
func (b *Bot) truncateMessage(str string) string {
	truncateMsg := str
	if len(str) > 4095 { // telegram API can only support 4096 bytes per message
		level.Warn(b.logger).Log("msg", "Message is bigger than 4095, truncate...")
		// find the end of last alert, we do not want break the html tags
		i := strings.LastIndex(str[0:4080], "\n\n") // 4080 + "\n<b>[SNIP]</b>" == 4095
		if i > 1 {
			truncateMsg = str[0:i] + "\n<b>[SNIP]</b>"
		} else {
			truncateMsg = "Message is too long... can't send.."
			level.Warn(b.logger).Log("msg", "truncateMessage: Unable to find the end of last alert.")
		}
		return truncateMsg
	}
	return truncateMsg
}

func parseUnmuteCommand(text string) ([]string, []string, error) {
	return parseCommands(text, ProjectAndEnvironmentUnmuteRegexp, UnmuteEnvironmentRegexp, UnmuteProjectRegexp)
}

func parseMuteCommand(text string) ([]string, []string ,error) {
	return parseCommands(text, ProjectAndEnvironmentMuteRegexp, MuteEnvironmentRegexp, MuteProjectRegexp)
}

func parseCommands(text string, projectAndEnvironmentRegexp string, environmentRegexp string,
	projectRegexp string) ([]string, []string, error) {
	matchProjectAndEnvironment, err := regexp.MatchString(projectAndEnvironmentRegexp, text)
	if err != nil {
		return []string{}, []string{}, err
	}

	regexProject, err := regexp.Compile(ProjectValuesRegexp)
	regexEnvironment, err := regexp.Compile(EnvironmentValuesRegexp)

	if matchProjectAndEnvironment {
		env := strings.Replace(regexEnvironment.FindStringSubmatch(text)[1], " ", "", -1)
		environmentsToMute := strings.Split(env, ",")

		p := strings.Replace(regexProject.FindStringSubmatch(text)[1], " ", "", -1)
		projectsToMute := strings.Split(p, ",")
		return environmentsToMute, projectsToMute, nil
	}

	matchEnvironment, err := regexp.MatchString(environmentRegexp, text)
	if matchEnvironment {
		env := strings.Replace(regexEnvironment.FindStringSubmatch(text)[1], " ", "", -1)
		environmentsToMute := strings.Split(env, ",")
		return environmentsToMute, []string{}, nil
	}

	matchProject, err := regexp.MatchString(projectRegexp, text)
	if matchProject {
		p := strings.Replace(regexProject.FindStringSubmatch(text)[1], " ", "", -1)
		projectsToRemove := strings.Split(p, ",")
		return []string{}, projectsToRemove, nil
	}

	return []string{}, []string{}, errors.New("no matches were found")
}

func arrayDifference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}
