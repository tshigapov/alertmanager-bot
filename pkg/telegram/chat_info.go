package telegram

import (
	"gopkg.in/tucnak/telebot.v2"
	"strings"
)

type ChatInfo struct {
	Chat				*telebot.Chat
	AlertEnvironments	[]string
	AlertProjects		[]string
	MutedEnvironments	[]string
	MutedProjects		[]string
}

func (ch *ChatInfo) UnmuteEnvironment(env string, allEnvs []string) {
	var index int
	for i, value := range ch.MutedEnvironments {
		if 0 == strings.Compare(value, env) {
			index = i
			break
		}
	}
	ch.MutedEnvironments = append(ch.MutedEnvironments[:index], ch.MutedEnvironments[index+1:]...)
	ch.AlertEnvironments = arrayDifference(allEnvs, ch.MutedEnvironments)
}

func (ch *ChatInfo) UnmuteProject(pr string, allPrs []string) {
	var index int
	for i, value := range ch.MutedProjects {
		if 0 == strings.Compare(value, pr) {
			index = i
			break
		}
	}
	ch.MutedProjects = append(ch.MutedProjects[:index], ch.MutedProjects[index+1:]...)
	ch.AlertProjects = arrayDifference(allPrs, ch.MutedProjects)
}

func (ch *ChatInfo) MuteEnvironments(envsToMute []string, allEnvs []string) {
	ch.MutedEnvironments = getUniqueStrings(append(ch.MutedEnvironments, envsToMute...))
	ch.AlertEnvironments = arrayDifference(allEnvs, ch.MutedEnvironments)
}

func (ch *ChatInfo) MuteProjects(prsToMute []string, allPrs []string) {
	ch.MutedProjects = getUniqueStrings(append(ch.MutedProjects, prsToMute...))
	ch.AlertProjects = arrayDifference(allPrs, ch.MutedProjects)
}

func getUniqueStrings(values []string) []string {
	uniqueSet := make(map[string]bool, len(values))
	for _, x := range values {
		uniqueSet[x] = true
	}
	uniqueValues := make([]string, 0, len(uniqueSet))
	for x := range uniqueSet {
		uniqueValues = append(uniqueValues, x)
	}
	return uniqueValues
}