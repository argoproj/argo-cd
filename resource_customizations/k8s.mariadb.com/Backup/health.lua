local health_status = {}

if obj.status ~= nil and obj.status.conditions ~= nil then

    for i, condition in ipairs(obj.status.conditions) do

        health_status.message = condition.reason .. " " .. condition.message

        if condition.status == "False" then
            if condition.reason == "CronJobScheduled" and condition.message == "Failed" then
                health_status.status = "Degraded"
                return health_status
            end
            health_status.status = "Progressing"
            return health_status
        end
    end

    health_status.status = "Healthy"
    return health_status
end

health_status.status = "Progressing"
health_status.message = "No status info available"
return health_status
