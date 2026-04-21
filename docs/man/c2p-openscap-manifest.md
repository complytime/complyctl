% C2P-OPENSCAP-MANIFEST.JSON(5) complyctl OpenSCAP Provider Configuration
% Marcus Burghardt <maburgha@redhat.com>
% June 2025

# NAME

c2p-openscap-manifest.json - Configuration file for the OpenSCAP provider used by complyctl

# DESCRIPTION

This file defines the metadata and runtime configuration options for the `openscap-provider`, a provider to be used with `complyctl`.

It is a JSON-formatted file typically installed at:

**/usr/share/complyctl/providers/c2p-openscap-manifest.json**

Some configuration options used by `openscap-provider` can be overridden by using a drop-in file with the same name in "`/etc/complyctl/config.d/`":

**/etc/complyctl/config.d/c2p-openscap-manifest.json**

The easiest way to create a drop-in file is copying **/usr/share/complyctl/providers/c2p-openscap-manifest.json** and defining the `default` values. Any other content can be removed to keep the drop-in file clean. See **CONFIGURATION OPTIONS** and **EXAMPLES** sections for more details.

For some specific cases, it is also possible to inform a custom configuration directory to override `/etc/complyctl/config.d`.
For example, the following command will try to locate and read custom settings from manifest files hosted in `/tmp/providers-conf` instead of `/etc/complyctl/config.d`:

`complyctl generate --provider-config /tmp/providers-conf`

See complyctl(1) for more details about the available options.

# FILE FORMAT

The configuration is a single JSON object with the following top-level keys:

- `metadata`: General provider information
- `executablePath`: Name or path of the provider binary
- `sha256`: The checksum of the binary (used for integrity checks)
- `configuration`: An array of runtime configuration options

# FIELDS

## metadata

```json
{
  "id": "openscap",
  "description": "My openscap provider",
  "version": "0.0.1",
  "types": [ "pvp" ]
}
```

## executablePath

Path or name of the provider binary to execute. Typically just:

```json
"executablePath": "openscap-provider"
```

## sha256
SHA256 checksum of the provider binary, used for runtime verification.

## configuration
A list of supported configuration parameters for the provider.

Each entry includes:

- name: The name of the parameter
- description: Explanation of its purpose
- required: Whether this parameter must be provided
- default (optional): The default value if not specified

# CONFIGURATION OPTIONS

## workspace (required)
Directory for writing provider artifacts. The value is inherited from complyctl and cannot be modified.

## profile (required)
The OpenSCAP profile to run for assessment. The value is inherited from complyctl and cannot be modified.

## datastream (optional)
The OpenSCAP datastream to use. If not set, the provider will try to determine it based on system information.

## results (optional, default: results.xml)
The name of the generated results file.

## arf (optional, default: arf.xml)
The name of the generated ARF file.

## policy (optional, default: tailoring_policy.xml)
The name of the generated tailoring file.

# EXAMPLES

This is an example of a manifest including all information.

```json
{
  "metadata": {
    "id": "openscap",
    "description": "My openscap provider",
    "version": "0.0.1",
    "types": [
      "pvp"
    ]
  },
  "executablePath": "openscap-provider",
  "sha256": "17e8d0b82c9bfbe7c195505090954488175005898fc0e8da0812c112c582426c",
  "configuration": [
    {
      "name": "workspace",
      "description": "Directory for writing provider artifacts",
      "required": true
    },
    {
      "name": "profile",
      "description": "The OpenSCAP profile to run for assessment",
      "required": true
    },
    {
      "name": "datastream",
      "description": "The OpenSCAP datastream to use. If not set, the provider will try to determine it based on system information",
      "required": false
    },
    {
      "name": "policy",
      "description": "The name of the generated tailoring file",
      "default": "tailoring_policy.xml",
      "required": false
    },
    {
      "name": "arf",
      "description": "The name of the generated ARF file",
      "default": "arf.xml",
      "required": false
    },
    {
      "name": "results",
      "description": "The name of the generated results file",
      "default": "results.xml",
      "required": false
    }
  ]
}
```

This is an example of a drop-in file modifying the openscap files.
```json
{
  "configuration": [
    {
      "name": "policy",
      "default": "custom_tailoring_policy.xml",
    },
    {
      "name": "arf",
      "default": "custom_arf.xml",
    },
    {
      "name": "results",
      "default": "custom_results.xml",
    }
  ]
}
```

# SEE ALSO

complyctl(1), complyctl-openscap-provider(7)

See the Upstream project at https://github.com/complytime/complyctl for more detailed documentation.
