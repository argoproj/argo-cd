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
                hs.status = "Degraded"
                hs.message = "No replicas are ready"
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
        hs.status = "Progressing"
        hs.message = "Scale status not available"
    end
end
return hs
