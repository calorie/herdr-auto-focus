# herdr-auto-focus

A macOS-only [Herdr](https://herdr.dev/) plugin that focuses an agent after it
needs attention and system input has been idle for five seconds.

## Behavior

- Queues `blocked` and `done` agent status changes.
- Prioritizes `blocked` agents over `done` agents.
- Waits until keyboard and pointing-device input across macOS has been idle for
  the configured duration.
- Re-checks the agent immediately before focusing it.
- Drops notifications that have already settled or whose pane is already
  focused.
- Requires new user input before automatically focusing another queued agent.
- Does not change Herdr sound or toast settings.

Herdr does not expose unsent prompt text. If you pause longer than the idle
duration while composing text, the plugin can change focus. Increase
`idle_seconds` if that is too aggressive.

## Requirements

- macOS
- Herdr 0.7.0 or later
- Go 1.22 or later at install time

## Install

```bash
herdr plugin install calorie/herdr-auto-focus
```

Herdr previews the manifest and build command before installation. The plugin
is built once and then runs as a native executable.

## Configure

The default idle duration is five seconds. To change it, obtain the plugin
configuration directory:

```bash
herdr plugin config-dir calorie.herdr-auto-focus
```

Create `config.json` in that directory:

```json
{
  "idle_seconds": 10
}
```

`idle_seconds` must be an integer from 1 through 3600. Invalid configuration
stops the event handler and is reported in the Herdr plugin log.

## Update

Herdr plugin v1 does not have a separate update command. Reinstall from GitHub:

```bash
herdr plugin install calorie/herdr-auto-focus
```

## Disable or enable

```bash
herdr plugin disable calorie.herdr-auto-focus
herdr plugin enable calorie.herdr-auto-focus
```

## Uninstall

```bash
herdr plugin uninstall calorie/herdr-auto-focus
```

## Local development

```bash
go test ./...
go vet ./...
go build -trimpath -o bin/herdr-auto-focus ./cmd/herdr-auto-focus
herdr plugin link .
```

## License

[MIT](LICENSE)
