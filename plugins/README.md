# Plugins
Plugins are triggered by actions defined on `bolt.observer` site.

# structure
Each plugin must be in it's own directory. Directory name must match `action name` defined on `bolt.observer` site.
It must implement `Plugin` interface defined in `entities/plugin.go`.
