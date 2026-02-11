hs = {}
hs.status = "Progressing"
hs.message = "Waiting for OpenTelemetry Collector status"

if obj.status ~= nil then
    if obj.status.scale ~= nil and obj.status.scale.statusReplicas ~= nil then
        local statusReplicas = obj.status.scale.statusReplicas

        -- Parse the statusReplicas string (e.g., "2/2", "0/2", "1/3")
        local ready, total = string.match(statusReplicas, "(%d+)/(%d+)")

        if ready ~= nil and total ~= nil then
            ready = tonumber(ready)
            total = tonumber(total)

            if ready == 0 then
                -- DaemonSet desiredNumberScheduled is based on eligible nodes.
                -- 0/0 is valid when no nodes match; treat as Healthy.
                -- https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/#how-daemon-pods-are-scheduled
                if obj.spec ~= nil and obj.spec.mode == "daemonset" and total == 0 then
                    hs.status = "Healthy"
                    hs.message = "DaemonSet has no schedulable nodes (0/0)"
                else
                    hs.status = "Degraded"
                    hs.message = "No replicas are ready"
                end
            elseif ready == total then
                hs.status = "Healthy"
                hs.message = "All replicas are ready"
            else
                hs.status = "Progressing"
                hs.message = "Replicas are starting up"
            end
        else
            hs.status = "Progressing"
            hs.message = "Unable to parse replica status: " ..tostring(statusReplicas)
        end
    else
        if obj.spec.mode == "sidecar" then
            hs.status = "Healthy"
            hs.message = "Collector is running in sidecar mode"
        else
            hs.status = "Progressing"
            hs.message = "Scale status not available"
        end
    end
end
return hs
