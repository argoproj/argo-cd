-- Surface deletion progress while the resource is terminating. You can customize this
-- block, e.g. map known finalizers in obj.metadata.finalizers to clearer messages.
if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil then
  local deletionHs = {}
  deletionHs.status = "Progressing"
  deletionHs.message = "Pending deletion"
  if obj.metadata.finalizers ~= nil and #obj.metadata.finalizers > 0 then
    deletionHs.message = "Pending deletion; blocked by finalizers: " .. table.concat(obj.metadata.finalizers, ", ")
  end
  return deletionHs
end

-- Reference CRD can be found here:
-- https://doc.crds.dev/github.com/kubernetes-sigs/cluster-api/cluster.x-k8s.io/MachinePool/v1beta1@v1.8.1

function getStatusBasedOnPhase(obj, hs)
    -- Phases can be found here: 
    -- https://github.com/kubernetes-sigs/cluster-api/blob/release-1.8/exp/api/v1beta1/machinepool_types.go#L139-L182
    if obj.status ~= nil and obj.status.phase ~= nil then
        hs.message = "MachinePool is " .. obj.status.phase
        if obj.status.phase == "Running" or obj.status.phase == "Scaling" then
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
