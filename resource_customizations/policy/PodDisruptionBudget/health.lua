-- Reference CRD can be found here:
-- https://kubernetes.io/docs/reference/kubernetes-api/policy-resources/pod-disruption-budget-v1/
hs = {}
hs.status = "Progressing"
hs.message = "Waiting for status"

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      -- InsufficientPods can have valid use cases
      -- See a discussion in https://github.com/argoproj/argo-cd/issues/20171 for more details
      if condition.status == "False" and condition.reason ~= "InsufficientPods" then
        hs.status = "Degraded"
        hs.message = "PodDisruptionBudget has " .. condition.reason
        return hs
      else
        hs.status = "Healthy"
        hs.message = "PodDisruptionBudget has " .. condition.reason
      end
    end
  end
end

return hs
