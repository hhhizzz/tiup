#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/weed volume \
{{- else}}
exec bin/weed volume \
{{- end}}
    -port={{.Port}} \
    -ip="{{.IPBind}}" \
    -mserver="{{.MasterAddr}}" \
    -dir="{{.Dirs}}" \
{{- if .DiskTypes}}
    -disk="{{.DiskTypes}}" \
{{- end}}
{{- if .DataCenter}}
    -dataCenter="{{.DataCenter}}" \
{{- end}}
{{- if .Rack}}
    -rack="{{.Rack}}" \
{{- end}}
    >> "{{.LogDir}}/seaweedfs_volume_stdout.log" 2>> "{{.LogDir}}/seaweedfs_volume_stderr.log"
