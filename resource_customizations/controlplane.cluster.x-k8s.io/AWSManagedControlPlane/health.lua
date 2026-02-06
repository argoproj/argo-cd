local health_status = {
    status = "Progressing",
    message = "Provisioning..."
}

-- If .status is nil or doesn't have conditions, then the control plane is not ready
if obj.status == nil or obj.status.conditions == nil then
    return health_status
end

-- Accumulator for the error messages (could be multiple conditions in error state)
err_msg = ""

-- Iterate over the conditions to determine the health status
for i, condition in ipairs(obj.status.conditions) do
    -- Check if the Ready condition is True, then the control plane is ready
    if condition.type == "Ready" and condition.status == "True" then
        health_status.status = "Healthy"
        health_status.message = "Control plane is ready"
        return health_status
    end

    -- If we have a condition that is False and has an Error severity, then the control plane is in a degraded state
    if condition.status == "False" and condition.severity == "Error" then
        health_status.status = "Degraded"
        err_msg = err_msg .. condition.message .. " "
    end
end

-- If we have any error conditions, then the control plane is in a degraded state
if health_status.status == "Degraded" then
    health_status.message = err_msg
    return health_status
end

-- If .status.ready is False, then the control plane is not ready
if obj.status.ready == false then
    health_status.status = "Progressing"
    health_status.message = "Control plane is not ready (.status.ready is false)"
    return health_status
end

-- If we reach this point, then the control plane is not ready and we don't have any error conditions
health_status.status = "Progressing"
health_status.message = "Control plane is not ready"

return health_status
