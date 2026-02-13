# Plugin Manifest Schema: c2p-ampel-manifest.json

Location: `~/.local/share/complytime/plugins/c2p-ampel-manifest.json`

## Schema

```json
{
  "metadata": {
    "id": "ampel",
    "description": "AMPEL Branch Protection Plugin for complyctl",
    "version": "0.1.0",
    "types": ["pvp"]
  },
  "executablePath": "ampel-plugin",
  "sha256": "<binary-hash>",
  "configuration": [
    {
      "name": "workspace",
      "description": "Directory for writing plugin artifacts",
      "required": true
    },
    {
      "name": "profile",
      "description": "The compliance profile to use",
      "required": true
    },
    {
      "name": "policy_dir",
      "description": "Directory for AMPEL policy files",
      "required": false,
      "default": "policy"
    },
    {
      "name": "results_dir",
      "description": "Directory for per-repository result files",
      "required": false,
      "default": "results"
    },
    {
      "name": "targets_file",
      "description": "Path to target repository configuration",
      "required": false,
      "default": "ampel-targets.yaml"
    }
  ]
}
```

## Configuration Resolution

1. `workspace` and `profile` are always provided by complyctl
2. Optional fields use defaults if not overridden
3. User overrides via `/etc/complytime/config.d/c2p-ampel-manifest.json`
4. Relative paths in `policy_dir`, `results_dir`, `targets_file`
   are resolved relative to `{workspace}/ampel/`
5. Absolute paths are used as-is

## Configuration Map Received by Configure()

After manifest resolution, the plugin receives:

```go
map[string]string{
    "workspace":    "/home/user/complytime",
    "profile":      "branch-protection-baseline",
    "policy_dir":   "policy",
    "results_dir":  "results",
    "targets_file": "ampel-targets.yaml",
}
```
