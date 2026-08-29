# {{ .name }}

{{- if not .items }}
{{ classSourceMarkdown .description .subclass_1 .subclass_2 }}
{{- else }}
{{ .description }}

---

- **DOMAINS -** [{{ .domain_1 }}](../domains/{{ urlEncode .domain_1 }}.md) & [{{ .domain_2 }}](../domains/{{ urlEncode .domain_2 }}.md)
- **STARTING EVASION -** {{ .evasion }}
- **STARTING HIT POINTS -** {{ .hp }}
- **CLASS ITEMS -** {{ .items }}

---

- **SUGGESTED TRAITS -** {{ .suggested_traits }}
- **SUGGESTED PRIMARY -** {{ .suggested_primary }}
  {{- if .suggested_secondary }}
- **SUGGESTED SECONDARY -** {{ .suggested_secondary }}
  {{- end }}
- **SUGGESTED ARMOR -** {{ .suggested_armor }}

---

### HOPE FEATURE

**_{{ .hope_feature_name }}:_** {{ mechanicsText .hope_feature_text }}

### CLASS FEATURE{{ if gt (len .feature) 1 }}S{{ end }}

{{ range .feature }}

**_{{ .name }}:_** {{ mechanicsText .text }}
{{ end }}

{{- if and (eq .name "Druid") .beastform_tiers }}

### BEASTFORM OPTIONS

When you use your "Beastform" feature, choose a creature category of your tier or lower. At the GM's discretion, you can describe yourself transforming into any animal that reasonably fits into that category.

Beastform categories are divided by tier. Each entry includes the following details:

- **Creature Category:** Each category's name describes the common role or behavior of creatures in that category (such as Agile Scout). This name is followed by a few examples of animals that fit in that category (in this example, fox, mouse, and weasel).
- **Character Trait:** While transformed, you gain a bonus to the listed trait. For example, while transformed into an Agile Scout, you gain a +1 bonus to your Agility. When this form drops, you lose this bonus.
- **Attack Rolls:** When you make an attack while transformed, you use the creature's listed range, trait, and damage dice, but you use your Proficiency. For example, as an Agile Scout, you can attack a target within Melee range using your Agility. On a success, you deal d4 physical damage using your Proficiency.
- **Evasion:** While transformed, you add the creature's Evasion bonus to your normal Evasion. For example, if your Evasion is usually 8 and your Beastform says "Evasion +2," your Evasion becomes 10 while you're in that form.
- **Advantage:** Your form makes you especially suited to certain actions. When you make an action or reaction roll related to one of the verbs listed for that creature category, you gain advantage on that roll. For example, an Agile Scout gains advantage on rolls made to sneak around, search for objects or creatures, and related activities.
- **Features:** Each form includes unique features. For example, an Agile Scout excels at silent, dexterous movement--but they're also fragile, making you more likely to drop out of Beastform.

### BEASTFORMS

{{- range .beastform_tiers }}

#### TIER {{ .tier }}

{{- range .items }}

- [{{ .name }}](../beastforms/{{ urlEncode (fileName .name) }}.md)
  {{- end }}
  {{ end }}
  {{- end }}

### SUBCLASSES

Choose either the **[{{ .subclass_1 }}](../subclasses/{{ urlEncode .subclass_1 }}.md)** or **[{{ .subclass_2 }}](../subclasses/{{ urlEncode .subclass_2 }}.md)** subclass.

### BACKGROUND QUESTIONS

_Answer any of the following background questions. You can also create your own questions._
{{- range .background }}

- {{ .question }}
  {{- end }}

### CONNECTIONS

_Ask your fellow players one of the following questions for their character to answer, or create your own questions._

{{- range .connection }}

- {{ .question }}
  {{- end }}
{{- end }}
