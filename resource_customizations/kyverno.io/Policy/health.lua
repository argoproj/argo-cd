-- Policy is the namespaced counterpart of ClusterPolicy and uses the same
-- Ready condition, so the health mapping matches kyverno.io/ClusterPolicy:
--   Ready=True   => Healthy
--   Ready=False  => Degraded
--   No status yet => Progressing
local hs = {}

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Ready" then
      if condition.status == "True" then
        hs.status = "Healthy"
        hs.message = "Policy is ready"
      else
        hs.status = "Degraded"
        hs.message = condition.message
      end
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Policy to be ready"
return hs
