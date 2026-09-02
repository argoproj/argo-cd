hs = {
    status = "Progressing",
    message = "Update in progress"
}
if (obj.metadata.labels ~= nil and obj.metadata.labels["shoot.gardener.cloud/status"] ~= nil) then
    if obj.metadata.labels["shoot.gardener.cloud/status"] == "healthy" then
        hs.status = "Healthy"
        hs.message = "Component state: Healthy"
    end
    if obj.metadata.labels["shoot.gardener.cloud/status"] == "progressing" then
        hs.status = "Progressing"
        hs.message = "Component state: Update in progress"
    end
    if obj.metadata.labels["shoot.gardener.cloud/status"] == "unhealthy" then
        hs.status = "Degraded"
        hs.message = "Component state: Unhealthy"
    end
    if obj.metadata.labels["shoot.gardener.cloud/status"] == "unknown" then
        hs.status = "Unknown"
        hs.message = "Component state: Unknown"
    end
    return hs
end
return hs
