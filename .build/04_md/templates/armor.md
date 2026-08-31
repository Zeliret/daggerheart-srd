---
Tier: {{ .tier }}
Thresholds: {{ yamlText .base_thresholds }}
Score: {{ .base_score }}
Feature: {{ yamlText (featureProperty .feature) }}
---

# {{ .name }}

**_Tier {{ .tier }}_** _Armor_

- **Base Thresholds:** {{ .base_thresholds }}
- **Base Score:** {{ .base_score }}

{{- if gt (len .feature) 0 }}
{{- $feat := index .feature 0 }}

### FEATURE

**_{{ $feat.name }}:_** {{ $feat.text }}
{{- end }}
