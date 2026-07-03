local health_status = {}
health_status.status = "Progressing"
health_status.message = "No status info available"

if obj.status ~= nil and obj.status.conditions ~= nil then

    for i, condition in ipairs(obj.status.conditions) do

        health_status.message = condition.reason .. " " .. condition.message
        if condition.reason == "JobComplete" and condition.status == "True" then
            health_status.status = "Healthy"
            return health_status
        end

        if condition.reason == "JobFailed" and condition.status == "True" then
            health_status.status = "Degraded"
            return health_status
        end
    end
end
return health_status
