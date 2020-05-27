# Bot for Prometheus' Alertmanager

This is the [Alertmanager](https://prometheus.io/docs/alerting/alertmanager/) bot for
[Prometheus](https://prometheus.io/) that notifies you on alerts.  
Just configure the Alertmanager to send Webhooks to the bot and that's it.

This version based on the [metalmatze/alertmanager-bot](https://github.com/metalmatze/alertmanager-bot). All rights reserved.

Additionally you can always **send commands** to get up-to-date information from the alertmanager.

### Changes
- Project uses a new version of Telegram Bot library - [telebot](https://github.com/tucnak/telebot)
- You can mute alerts from different environments and projects
- Get list of environments and projects that not muted 
- Bot can delete alert messages in a specified period of time

### Why?

Alertmanager already integrates a lot of different messengers as receivers for alerts.  
I want to extend this basic functionality.

Previously the Alertmanager could only talk to you via a chat, but now you can talk back via [commands](#commands).  
You can ask about current ongoing [alerts](#alerts) and [silences](#silences) and mute [environments](#environments) and 
[projects](#projects).
  

## Messengers

Supports only [Telegram](https://telegram.org/).

## Commands

###### /start

> Hey, Matthias! I will now keep you up to date!  
> [/help](#help)

###### /stop

> Alright, Matthias! I won't talk to you again.  
> [/help](#help)

###### /alerts

> 🔥 **FIRING** 🔥  
> **NodeDown** (Node scraper.krautreporter:8080 down)  
> scraper.krautreporter:8080 has been down for more than 1 minute.  
> **Started**: 1 week 2 days 3 hours 50 minutes 42 seconds ago  
>
> 🔥 **FIRING** 🔥
> **monitored_service_down** (MONITORED SERVICE DOWN)
> The monitoring service 'digitalocean-exporter' is down.
> **Started**: 10 seconds ago

###### /silences

> NodeDown 🔕  
>  `job="ranch-eye" monitor="exporter-metrics" severity="page"`  
> **Started**: 1 month 1 week 5 days 8 hours 27 minutes 57 seconds ago  
> **Ends**: -11 months 2 weeks 2 days 19 hours 15 minutes 24 seconds  
>
> RancherServiceState 🔕  
>  `job="rancher" monitor="exporter-metrics" name="scraper" rancherURL="http://rancher.example.com/v1" severity="page" state="inactive"`  
> **Started**: 1 week 2 days 3 hours 46 minutes 21 seconds ago  
> **Ends**: -3 weeks 1 day 13 minutes 24 seconds  

###### /chats

> Currently these chat have subscribed:
> @MetalMatze


###### /status

> **AlertManager**  
> Version: 0.5.1  
> Uptime: 3 weeks 1 day 6 hours 15 minutes 2 seconds  
> **AlertManager Bot**  
> Version: 0.4.0  
> Uptime: 3 weeks 1 hour 17 minutes 19 seconds  


###### /mute
> You were successfully muted environments and/or projects

Command examples:
- `/mute environment[env1, env2]`
- `/mute project[pr1, pr2]`
- `/mute environment[env1], project[pr2]` 

###### /mute_del
> You were successfully delete mute from environments and/or projects

###### /environments
> The following environments are available: [env1 env2 env3]

###### /projects
> The following projects are available: [pr1, pr2]

###### /muted_envs
> Muted environments: [env1 env4]

###### /muted_prs
> Muted projects: [pr2 pr5]

###### /help

> I'm a Prometheus AlertManager Bot for Telegram. I will notify you about alerts.  
> You can also ask me about my [/status](#status), [/alerts](#alerts) & [/silences](#silences)  
>   
> Available commands:  
> [/start](#start) - Subscribe for alerts.  
> [/stop](#stop) - Unsubscribe for alerts.  
> [/status](#status) - Print the current status.  
> [/alerts](#alerts) - List all alerts.  
> [/silences](#silences) - List all silences.  
> [/chats](#chats) - List all users and group chats that subscribed.
> [/mute](#mute) - Mute environments and/or projects.
> [/mute_del](#mute_del) - Delete mute for environments/projects.
> [/environments](#environments) - List all environments for alerts.
> [/projects](#projects) - List all projects for alerts.
> [/muted_envs](#muted_envs) - List all muted environments.
> [/muted_prs](#muted_prs) - List all muted projects.

## Installation

### Docker

`docker pull kgusman/alertmanager-bot:1.1.0`

Start as a command:

#### Bolt Storage

```bash
docker run -d \
	-e 'ALERTMANAGER_URL=http://alertmanager:9093' \
	-e 'BOLT_PATH=/data/bot.db' \
	-e 'STORE=bolt' \
	-e 'TELEGRAM_ADMIN=1234567' \
	-e 'TELEGRAM_TOKEN=XXX' \
	-e 'PROMETHEUS_ENVS=env1, env2, env3' \
	-e 'PROMETHEUS_PROJECTS=pr1, pr2' \
    -e 'FETCH_PERIOD=2' \
    -e 'DELETE_PERIOD=1' \
	-v '/srv/monitoring/alertmanager-bot:/data' \
	--name alertmanager-bot \
	kgusman/alertmanager-bot:1.1.0
```

#### Consul Storage

```bash
docker run -d \
	-e 'ALERTMANAGER_URL=http://alertmanager:9093' \
	-e 'CONSUL_URL=localhost:8500' \
	-e 'STORE=consul' \
	-e 'TELEGRAM_ADMIN=1234567' \
	-e 'TELEGRAM_TOKEN=XXX' \
    -e 'PROMETHEUS_ENVS=env1, env2, env3' \
	-e 'PROMETHEUS_PROJECTS=pr1, pr2' \
	-e 'FETCH_PERIOD=2' \
    -e 'DELETE_PERIOD=1' \
    --name alertmanager-bot \
    kgusman/alertmanager-bot:1.1.0
```

Usage within docker-compose:

```yml
alertmanager-bot:
  image: kgusman/alertmanager-bot:1.1.0
  environment:
    ALERTMANAGER_URL: http://alertmanager:9093
    BOLT_PATH: /data/bot.db
    STORE: bolt
    TELEGRAM_ADMIN: '1234567'
    TELEGRAM_TOKEN: XXX
    PROMETHEUS_ENVS: env1, env2, env3
    PROMETHEUS_PROJECTS: pr1, pr2
    FETCH_PERIOD: 2
    DELETE_PERIOD: 1
    TEMPLATE_PATHS: /templates/default.tmpl
  volumes:
  - /srv/monitoring/alertmanager-bot:/data
```

### Ansible

If you prefer using configuration management systems (like Ansible) you might be interested in the following role:  [mbaran0v.alertmanager-bot](https://github.com/mbaran0v/ansible-role-alertmanager-bot)

### Configuration

ENV Variable | Description
|---------------------|------------------------------------------------------|
| ALERTMANAGER_URL    | Address of the alertmanager, default: `http://localhost:9093` |
| BOLT_PATH           | Path on disk to the file where the boltdb is stored, default: `/tmp/bot.db` |
| CONSUL_URL          | The URL to use to connect with Consul, default: `localhost:8500` |
| LISTEN_ADDR         | Address that the bot listens for webhooks, default: `0.0.0.0:8080` |
| STORE               | The type of the store to use, choose from bolt (local) or consul (distributed) |
| TELEGRAM_ADMIN      | The Telegram user id for the admin. The bot will only reply to messages sent from an admin. All other messages are dropped and logged on the bot's console. |
| TELEGRAM_TOKEN      | Token you get from [@botfather](https://telegram.me/botfather) |
| PROMETHEUS_ENVS     | List of environments monitored by Prometheus. String with comma-separated values |
| PROMETHEUS_PROJECTS | List of projects monitored by Prometheus. String with comma-separated values  |
| FETCH_PERIOD        | Scheduler period for fetching messages from store (in minutes) |
| DELETE_PERIOD       | Time after messages have to be deleted (in minutes) |
| TEMPLATE_PATHS      | Path to custom message templates, default template is `./default.tmpl`, in docker - `/templates/default.tmpl` |

#### Authentication

Additional users may be allowed to command the bot by giving multiple instances
of the `--telegram.admin` command line option or by specifying a
newline-separated list of telegram user IDs in the `TELEGRAM_ADMIN` environment
variable.
```
- TELEGRAM_ADMIN="**********\n************"
--telegram.admin=1 --telegram.admin=2
```
#### Alertmanager Configuration

Now you need to connect the Alertmanager to send alerts to the bot.  
A webhook is used for that, so make sure your `LISTEN_ADDR` is reachable for the Alertmanager.

For example add this to your `alertmanager.yml` configuration:
```yaml
receivers:
- name: 'alertmananger-bot'
  webhook_configs:
  - send_resolved: true
    url: 'http://alertmanager-bot:8080'
```

## Development

Get all dependencies. We use [golang/dep](https://github.com/golang/dep).  
Fetch all dependencies with:

```
dep ensure -v -vendor-only
```

Build the binary using `make`:

```
make install
```

In case you have `$GOPATH/bin` in your `$PATH` you can now simply start the bot by running:

```bash
alertmanager-bot
```
