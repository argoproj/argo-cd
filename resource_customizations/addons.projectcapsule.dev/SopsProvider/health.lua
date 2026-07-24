-- SopsProvider is a cluster-scoped resource that configures a decryption provider
-- (e.g. AWS KMS, GCP KMS, age) for the SOPS Operator.
--
-- Documentation:
--   https://github.com/peak-scale/sops-operator
--
-- ArgoCD health mapping:
--   Ready=True    => Healthy
--   Ready=False   => Degraded
--   Ready=Unknown => Progressing
--   No status     => Progressing
local hs = {}
if obj.status == nil or obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for status"
  return hs
end

if obj.metadata ~= nil and obj.metadata.generation ~= nil and obj.status.observedGeneration ~= nil
    and obj.status.observedGeneration ~= obj.metadata.generation then
  hs.status = "Progressing"
  hs.message = "Waiting for reconciliation (generation mismatch)"
  return hs
end

for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "Ready" then
    if condition.status == "True" then
      hs.status = "Healthy"
      hs.message = condition.message
      return hs
    end
    if condition.status == "False" then
      hs.status = "Degraded"
      hs.message = condition.message
      return hs
    end
    if condition.status == "Unknown" then
      hs.status = "Progressing"
      hs.message = condition.message
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Ready condition"
return hs
