# complyctl

[![OpenSSF Best Practices status](https://www.bestpractices.dev/projects/9761/badge)](https://www.bestpractices.dev/projects/9761)
[![GoDoc](https://img.shields.io/static/v1?label=godoc&message=reference&color=blue)](https://pkg.go.dev/github.com/complytime/complyctl)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/complytime/complyctl/badge)](https://scorecard.dev/viewer/?uri=github.com/complyctl/complyctl)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=rh-psce_complyctl&metric=coverage)](https://sonarcloud.io/summary/new_code?id=rh-psce_complyctl)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=rh-psce_complyctl&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=rh-psce_complyctl)

ComplyCTL leverages [OSCAL](https://github.com/usnistgov/OSCAL/) to perform compliance assessment activities, using plugins for each stage of the lifecycle.

## Documentation

:paperclip: [Installation](./docs/INSTALLATION.md)\
:paperclip: [Quick Start](./docs/QUICK_START.md)\
:paperclip: [Sample Component Definition](./docs/samples/sample-component-definition.json)

### Basic Usage

Determine the baseline you want to run a scan for and create an OSCAL [Assessment Plan](https://pages.nist.gov/OSCAL/learn/concepts/layer/assessment/assessment-plan/). The Assessment
Plan will act as configuration to guide the complyctl generation and scanning operations.

### `list` command

```bash
complyctl list
...
# Table appears with options. Look at the Framework ID column.
```

### `info` command

```bash
complyctl info <framework-id>
# Display information about a framework's controls and rules.

complyctl info <framework-id> --control <control-id>
# Display details about a specific control.

complyctl info <framework-id> --rule <rule-id>
# Display details about a specific rule.

complyctl info <framework-id> --parameter <parameter-id>
# Display details about a specific parameter.
```

### `plan` command

```bash
complyctl plan <framework-id>
...
# The file will be written out to assessment-plan.json in the specified workspace.
# Defaults to current working directory.

cat complytime/assessment-plan.json
# The default assessment-plan.json will be available in the complytime workspace (complytime/assessment-plan.json).

complyctl plan <framework-id> --dry-run
# See the default contents of the assessment-plan.json.
```

Use a scope config file to customize the assessment plan:

```bash
complyctl plan <framework-id> --dry-run --out config.yml
# Customize the assessment-plan.json with the 'out' flag. Updates can be made to the config.yml.
```

Open the `config.yml` file in a text editor and modify the YAML as desired.  The example below shows various options for including and excluding rules.

The `selectParameters` YAML key sets parameters for the `controlId`. If you try to use a value that isn't supported, an error will occur, and the valid alternative values will be displayed. To fix this, update the `value` in the `config.yml` file, and then run the command with the `--scope-config <config.yml>` flag. This will generate a new `assessment-plan.json` file with the updated values.

```yaml
frameworkId: example-framework
includeControls:
- controlId: control-01
  controlTitle: Title of Control 01
  includeRules:
  - "*" # all rules included by default
  selectParameters:
  - name: param-1-id
    value: param-1-value
  - name: param-2-id
    value: param-2-value  
- controlId: control-02
  controlTitle: Title of Control 02
  includeRules:
  - "rule-02" # only rule-02 will be included for this control
- controlId: control-03
  controlTitle: Title of Control 03
  includeRules:
  - "*"
  selectParameters:
  - name: param-1-id
    value: param-1-value
  - name: param-5-id
    value: param-5-value # update the value with available alternatives
  excludeRules:
  - "rule-03" # exclude rule-03 specific rule from control-03
globalExcludeRules:
  - "rule-99" # will be excluded for all controls, this takes priority over any includeRules clauses above
```

The edited `config.yml` can then be used with the `plan` command to customize the assessment plan.

```bash
complyctl plan <framework-id> --scope-config config.yml
# The config.yml will be loaded by passing '--scope-config' to customize the assessment-plan.json.
```

### `generate` command

```bash
complyctl generate
# Run the `generate` command to generate the plugin specific policy artifacts in the workspace.
```

### `scan` command

```bash
complyctl scan
# Run the `scan` command to execute the PVP plugins and create results artifacts. The results will be written to assessment-results.json in the specified workspace.

complyctl scan --with-md
# Results can also be created in Markdown format by passing the `--with-md` flag.
```

## Contributing

:paperclip: Read the [contributing guidelines](./docs/CONTRIBUTING.md)\
:paperclip: Read the [style guide](./docs/STYLE_GUIDE.md)\
:paperclip: Read and agree to the [Code of Conduct](./docs/CODE_OF_CONDUCT.md)

*Interested in writing a plugin?* See the [plugin guide](./docs/PLUGIN_GUIDE.md).
