local hs = {}

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Ready" and condition.status == "True" then
      hs.status = "Healthy"
      hs.message = "3scale CustomPolicyDefinition is ready"
      return hs
    elseif condition.type == "Invalid" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message or "3scale CustomPolicyDefinition configuration is invalid"
      return hs
    elseif condition.type == "Failed" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message or "3scale CustomPolicyDefinition synchronization failed"
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for 3scale CustomPolicyDefinition status..."
return hs
