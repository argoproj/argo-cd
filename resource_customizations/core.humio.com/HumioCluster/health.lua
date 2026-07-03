
hs = {
    status = "Progressing",
    message = "Update in progress"
}

if obj.status == nil then
    hs.status= "Progressing"
    if obj.status.message ~= nil then
        hs.message = obj.status.message
    end
end

if obj.status ~= nil then
    if obj.status.state ~= nil then
        if obj.status.state == "Running" then
            hs.status = "Healthy"
            if obj.status.message ~= nil then
                hs.message = obj.status.message
            else
                hs.message = "Cluster is in a healthy running state"
            end
        end
        if obj.status.state == "Restarting" then
            hs.status = "Progressing"
            if obj.status.message ~= nil then
                hs.message = obj.status.message
            else
                hs.message = "Cluster pods are being restarted"
            end
        end
        if obj.status.state == "Upgrading" then
            hs.status = "Progressing"
            if obj.status.message ~= nil then
                hs.message = obj.status.message
            else
                hs.message = "Cluster pods are being upgraded"
            end
        end
        if obj.status.state == "ConfigError" then
            hs.status = "Degraded"
            if obj.status.message ~= nil then
                hs.message = obj.status.message
            else
                hs.message = "User-provided cluster specification resulted in a configuration error"
            end
        end
        if obj.status.state == "Pending" then
            hs.status = "Progressing"
            if obj.status.message ~= nil then
                hs.message = obj.status.message
            else
                hs.message = "Cluster is waiting on resources to be provisioned"
            end
        end
        if obj.status.state == "Unknown" then
            hs.status = "Unknown"
            if obj.status.message ~= nil then
                hs.message = obj.status.message
            else
                hs.message = "Component state: Unknown."
            end
        end
    end
    return hs
end
return hs