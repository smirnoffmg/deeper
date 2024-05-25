# Deeper - easy-to-use OSINT tool

## Env variables

```bash
GITHUB_TOKEN=your_github_token
```

## How to use

```bash
go get
go run main.go {email/username/domain}
```

## How to extend

```go
package new_plugin

import (
 "github.com/smirnoffmg/deeper/internal/entities"
 "github.com/smirnoffmg/deeper/internal/state"
)

func init() {
 p := NewPlugin()
 p.Register()
}

type NewPlugin struct {}

func NewPlugin() *NewPlugin {
  return &NewPlugin{}
}

func (g *NewPlugin) Register() error {
 pluginEntityType := entities.Username  // for example, better use proper EntityType
 plugins := state.ActivePlugins[pluginEntityType]
 state.ActivePlugins[pluginEntityType] = append(plugins, g)

 return nil
}

func (g *NewPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
 traces := []entities.Trace
 return traces, nil

}

func (g NewPlugin) String() string {
 return "NewPlugin"
}

```
