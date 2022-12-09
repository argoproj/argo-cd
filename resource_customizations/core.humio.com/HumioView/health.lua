hs = {
    status = "Progressing",
    message = "Update in progress"
}
if obj.status ~= nil then
    if obj.status.state ~= nil then
        if obj.status.state == "Exists" then
            hs.status = "Healthy"
            hs.message = "Component state: Exists."
        end
        if obj.status.state == "NotFound" then
            hs.status = "Missing"
            hs.message = "Component state: NotFound."
        end
        if obj.status.state == "ConfigError" then
            hs.status = "Degraded"
            hs.message = "Component state: ConfigError."
        end
        if obj.status.state == "Unknown" then
            hs.status = "Unknown"
            hs.message = "Component state: Unknown."
        end
    end
    return hs
end
return hs
