local health_status = {}
if obj.status ~= nil then
    if obj.status.currentState ~= nil then
        if obj.status.currentState == "Initialized" then
            health_status.status = "Healthy"
            health_status.message = obj.status.currentState
            return health_status
        end
        if obj.status.currentState:find("^Provisioning") ~= nil then
            health_status.status = "Progressing"
            health_status.message = obj.status.currentState
            return health_status
        end
        if obj.status.currentState:find("^Waiting") ~= nil then
            health_status.status = "Progressing"
            health_status.message = obj.status.currentState
            return health_status
        end
        if obj.status.currentState:find("^Updating") ~= nil then
            health_status.status = "Progressing"
            health_status.message = obj.status.currentState
            return health_status
        end
        if obj.status.currentState == "Restarting MinIO" then
            health_status.status = "Progressing"
            health_status.message = obj.status.currentState
            return health_status
        end
        if obj.status.currentState == "Statefulset not controlled by operator" then
            health_status.status = "Degraded"
            health_status.message = obj.status.currentState
            return health_status
        end
        if obj.status.currentState == "Another MinIO Tenant already exists in the namespace" then
            health_status.status = "Degraded"
            health_status.message = obj.status.currentState
            return health_status
        end
        if obj.status.currentState == "Tenant credentials are not set properly" then
            health_status.status = "Degraded"
            health_status.message = obj.status.currentState
            return health_status
        end
        if obj.status.currentState == "Different versions across MinIO Pools" then
            health_status.status = "Degraded"
            health_status.message = obj.status.currentState
            return health_status
        end
        if obj.status.currentState == "Pool Decommissioning Not Allowed" then
            health_status.status = "Degraded"
            health_status.message = obj.status.currentState
            return health_status
        end
        health_status.status = "Progressing"
        health_status.message = obj.status.currentState
        return health_status
    end
end
health_status.status = "Progressing"
health_status.message = "No status info available"
return health_status
