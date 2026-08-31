# {{ .name }}

{{ sourceMarkdown .description }}

### DOMAIN CARDS

| **Level** | **Option 1** | **Option 2** | **Option 3** |
| :-------: | ------------ | ------------ | ------------ |

{{- range $index, $card := .card }}
| **{{ add1 $index }}** | {{ abilityLink (optionAt $card 1) }} | {{ abilityLink (optionAt $card 2) }} | {{ abilityLink (optionAt $card 3) }} |
{{- end }}
