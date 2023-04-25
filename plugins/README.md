# Plugins
Plugins are triggered by actions defined on `bolt.observer` site.

# Structure
Each plugin must be in it's own directory. Directory name must match `action name` defined on `bolt.observer` site.
It must implement `Plugin` interface defined in `entities/plugin.go`.

# Compile time enablement
We are using tag `plugins` so you can build agent without plugins too. By default plugins are enabled (`go build -tags=plugins`).
To make everything work in Visual Studio Code, go to Settings / Settings / Gopls / Edit settings.json and add

```
"go.testTags": "plugins",
"gopls": {
    "build.buildFlags": ["-tags=plugins"]
}
```

# Run-time control

There is an option `--noplugins` to disable all plugins even though you might have them compiled in the release.
