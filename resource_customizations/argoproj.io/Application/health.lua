health_status = {}
if obj.status ~= nil then
    if obj.status.health ~= nil then
        health_status.status = obj.status.health.status
        health_status.message = obj.status.health.message
    end
end
if health_status.status == nil then
    health_status.status = "Progressing"
    health_status.message = "Waiting for application to be reconciled"
end
return health_status