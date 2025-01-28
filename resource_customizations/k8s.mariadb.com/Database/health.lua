local health_status = {}
health_status.status = "Progressing"
health_status.message = "No status info available"

if obj.status ~= nil and obj.status.conditions ~= nil then

    for i, condition in ipairs(obj.status.conditions) do

        health_status.message = condition.message

        if condition.type == "Ready" then 
            if condition.status == "True" then
                health_status.status = "Healthy"
            else 
                health_status.status = "Degraded"
            end
            return health_status
        end
    end
end


return health_status
