#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/weed filer \
{{- else}}
exec bin/weed filer \
{{- end}}
    -port={{.Port}} \
    -ip="{{.IPBind}}" \
    -master="{{.MasterAddr}}" \
    >> "{{.LogDir}}/seaweedfs_filer_stdout.log" 2>> "{{.LogDir}}/seaweedfs_filer_stderr.log"
