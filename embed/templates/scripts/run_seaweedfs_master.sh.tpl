#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/weed master \
{{- else}}
exec bin/weed master \
{{- end}}
    -port={{.Port}} \
    -ip="{{.IP}}" \
{{- if .IPBind}}
    -ip.bind="{{.IPBind}}" \
{{- end}}
{{- if .Peers}}
    -peers="{{.Peers}}" \
{{- end}}
    -defaultReplication="{{.DefaultReplication}}" \
{{- if .VolumeSizeLimitMB}}
    -volumeSizeLimitMB={{.VolumeSizeLimitMB}} \
{{- end}}
    -mdir="{{.DataDir}}" \
    >> "{{.LogDir}}/seaweedfs_master_stdout.log" 2>> "{{.LogDir}}/seaweedfs_master_stderr.log"
