{{- define "frulab.labels" -}}
app.kubernetes.io/part-of: frulab
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}

{{- define "frulab.name" -}}{{ .Release.Name }}-frulab{{- end }}
{{- define "tester.name" -}}{{ .Release.Name }}-tester{{- end }}

{{- define "frulab.selector" -}}
app.kubernetes.io/name: frulab
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "tester.selector" -}}
app.kubernetes.io/name: fru-tester
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/* NAD name of one tester interface: (dict "root" $ "if" "n2") */}}
{{- define "tester.nad" -}}
{{- $i := index .root.Values.tester.network.macvlan.interfaces .if -}}
{{- if and (not .root.Values.tester.network.macvlan.create) $i.name -}}
{{ $i.name }}
{{- else -}}
{{ .root.Release.Name }}-{{ .if }}
{{- end -}}
{{- end }}

{{/* a data volume: (dict "name" "db" "p" .Values.x.persistence "claim" "...") */}}
{{- define "frulab.volume" -}}
- name: {{ .name }}
{{- if eq .p.type "hostPath" }}
  hostPath:
    path: {{ .p.hostPath }}
    type: DirectoryOrCreate
{{- else if eq .p.type "pvc" }}
  persistentVolumeClaim:
    claimName: {{ .claim }}
{{- else if eq .p.type "emptyDir" }}
  emptyDir: {}
{{- else }}
{{- fail (printf "persistence.type must be hostPath, pvc or emptyDir, not %q" .p.type) }}
{{- end }}
{{- end }}

{{- define "frulab.pvc" -}}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ .claim }}
  labels:
    {{- include "frulab.labels" .root | nindent 4 }}
  annotations:
    helm.sh/resource-policy: keep
spec:
  accessModes: [ReadWriteOnce]
  {{- with .p.storageClass }}
  storageClassName: {{ . }}
  {{- end }}
  resources:
    requests:
      storage: {{ .p.size }}
{{- end }}
