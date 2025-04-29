-- Reference CRD can be found here:
-- https://doc.crds.dev/github.com/kubernetes-sigs/cluster-api/cluster.x-k8s.io/MachinePool/v1beta1@v1.8.1

function getStatusBasedOnPhase(obj, hs)
    -- Phases can be found here: 
    -- https://github.com/kubernetes-sigs/cluster-api/blob/release-1.8/exp/api/v1beta1/machinepool_types.go#L139-L182
    if obj.status ~= nil and obj.status.phase ~= nil then
        hs.message = "MachinePool is " .. obj.status.phase
        if obj.status.phase == "Running" then
            hs.status = "Healthy"
        end
        if obj.status.phase == "Failed" or obj.status.phase == "Unknown"  then
            hs.status = "Degraded"
        end
    end
    return hs
end

function getConditionStatuses(obj, hs)
    local extraInfo = ""
    if obj.status ~= nil and obj.status.conditions ~= nil then
        for i, condition in ipairs(obj.status.conditions) do
            if condition.type ~= nil and condition.status == "False" then
                if extraInfo ~= "" then
                    extraInfo = extraInfo .. ", "
                end
                extraInfo = extraInfo .. "Not " .. condition.type
                if condition.reason ~= nil then
                    extraInfo = extraInfo .. " (" .. condition.reason .. ")"
                end
            end
        end
    end
    if extraInfo ~= "" then
        hs.message = hs.message .. ": " .. extraInfo
    end

    return hs
end

local hs = {}
hs.status = "Progressing"
hs.message = ""

getStatusBasedOnPhase(obj, hs)
getConditionStatuses(obj, hs)

return hs
