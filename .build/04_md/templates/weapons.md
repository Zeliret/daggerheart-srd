---
Tier: {{ .tier }}
Category: {{ yamlText .primary_or_secondary }}
Trait: {{ yamlText .trait }}
Range: {{ yamlText .range }}
Damage: {{ yamlText .damage }}
Burden: {{ yamlText .burden }}
Feature: {{ yamlText (featureProperty .feature) }}
---

# {{ .name }}

**_Tier {{ .tier }}_** _{{ .primary_or_secondary }}{{ if .physical_or_magical }}_ _{{ .physical_or_magical }}{{ end }}_ _Weapon_

- **Trait:** {{ .trait }}
- **Range:** {{ .range }}
- **Damage:** {{ .damage }}
- **Burden:** {{ .burden }}

{{- if gt (len .feature) 0 }}
{{- $feat := index .feature 0 }}

### FEATURE

**_{{ $feat.name }}:_** {{ $feat.text }}
{{- end }}
