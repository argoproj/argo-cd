{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "redis.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Expand the chart plus release name (used by the chart label)
*/}}
{{- define "redis.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "redis.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for networkpolicy.
*/}}
{{- define "networkPolicy.apiVersion" -}}
{{- if semverCompare ">=1.4-0, <1.7-0" .Capabilities.KubeVersion.GitVersion -}}
{{- print "extensions/v1beta1" -}}
{{- else -}}
{{- print "networking.k8s.io/v1" -}}
{{- end -}}
{{- end -}}

{{/*
Return the proper image name
*/}}
{{- define "redis.image" -}}
{{- $registryName :=  .Values.image.registry -}}
{{- $repositoryName := .Values.image.repository -}}
{{- $tag := .Values.image.tag | toString -}}
{{- printf "%s/%s:%s" $registryName $repositoryName $tag -}}
{{- end -}}

{{/*
Return the proper image name (for the metrics image)
*/}}
{{- define "metrics.image" -}}
{{- $registryName :=  .Values.metrics.image.registry -}}
{{- $repositoryName := .Values.metrics.image.repository -}}
{{- $tag := .Values.metrics.image.tag | toString -}}
{{- printf "%s/%s:%s" $registryName $repositoryName $tag -}}
{{- end -}}

{{/*
Return slave readiness probe
*/}}
{{- define "redis.slave.readinessProbe" -}}
{{- $readinessProbe := .Values.slave.readinessProbe | default .Values.master.readinessProbe -}}
{{- if $readinessProbe }}
{{- if $readinessProbe.enabled }}
readinessProbe:
  initialDelaySeconds: {{ $readinessProbe.initialDelaySeconds | default .Values.master.readinessProbe.initialDelaySeconds }}
  periodSeconds: {{ $readinessProbe.periodSeconds | default .Values.master.readinessProbe.periodSeconds }}
  timeoutSeconds: {{ $readinessProbe.timeoutSeconds | default .Values.master.readinessProbe.timeoutSeconds }}
  successThreshold: {{ $readinessProbe.successThreshold | default .Values.master.readinessProbe.successThreshold }}
  failureThreshold: {{ $readinessProbe.failureThreshold | default .Values.master.readinessProbe.failureThreshold }}
  exec:
    command:
    - redis-cli
    - ping
{{- end }}
{{- end -}}
{{- end -}}

{{/*
Return slave liveness probe
*/}}
{{- define "redis.slave.livenessProbe" -}}
{{- $livenessProbe := .Values.slave.livenessProbe | default .Values.master.livenessProbe -}}
{{- if $livenessProbe }}
{{- if $livenessProbe.enabled }}
livenessProbe:
  initialDelaySeconds: {{ $livenessProbe.initialDelaySeconds | default .Values.master.livenessProbe.initialDelaySeconds }}
  periodSeconds: {{ $livenessProbe.periodSeconds | default .Values.master.livenessProbe.periodSeconds }}
  timeoutSeconds: {{ $livenessProbe.timeoutSeconds | default .Values.master.livenessProbe.timeoutSeconds }}
  successThreshold: {{ $livenessProbe.successThreshold | default .Values.master.livenessProbe.successThreshold }}
  failureThreshold: {{ $livenessProbe.failureThreshold | default .Values.master.livenessProbe.failureThreshold}}
  exec:
    command:
    - redis-cli
    - ping
{{- end }}
{{- end -}}
{{- end -}}

{{/*
Return slave security context
*/}}
{{- define "redis.slave.securityContext" -}}
{{- $securityContext := .Values.slave.securityContext | default .Values.master.securityContext -}}
{{- if $securityContext }}
{{- if $securityContext.enabled }}
securityContext:
  fsGroup: {{ $securityContext.fsGroup | default .Values.master.securityContext.fsGroup }}
  runAsUser: {{ $securityContext.runAsUser | default .Values.master.securityContext.runAsUser }}
{{- end }}
{{- end }}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "redis.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ default (include "redis.fullname" .) .Values.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}
