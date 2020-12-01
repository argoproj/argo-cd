local json = require("json")
health_status = {
    status = "Progressing",
    message = "Waiting to Autoscale"
}

function is_degraded( condition )
    degraded_states = {
        {"AbleToScale", "FailedGetScale"},
        {"AbleToScale", "FailedUpdateScale"},
        {"ScalingActive", "FailedGetResourceMetric"},
        {"ScalingActive", "InvalidSelector"}
    }
    for i, state in ipairs(degraded_states) do
        if condition.type == state[1] and condition.reason == state[2] then
            return true
        end 
    end
    return false
end

condition_string = obj.metadata.annotations["autoscaling.alpha.kubernetes.io/conditions"]
if condition_string ~= nil then
    conditions = json.decode(condition_string)
end

if conditions then
    for i, condition in ipairs(conditions) do
        if is_degraded(condition) then
            health_status.status = "Degraded"
            health_status.message = condition.message
            return health_status
        end
        if condition.type == "AbleToScale" and  condition.reason == "SucceededRescale" then
            health_status.status = "Healthy"
            health_status.message = condition.message
            return health_status
        end
        if condition.type == "ScalingLimited" and  condition.reason == "DesiredWithinRange" then
            health_status.status = "Healthy"
            health_status.message = condition.message
            return health_status
        end
    end
end

return health_status
