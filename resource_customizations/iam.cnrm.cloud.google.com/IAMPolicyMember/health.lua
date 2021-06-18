hs = {}
hs.status = "Progressing"
hs.message = "Provisioning IAMPolicyMember..."

if obj.status == nil or obj.status.conditions == nil then
  return hs
end

for i, condition in ipairs(obj.status.conditions) do
  -- There should be only Ready status
  if condition.type == "Ready" then

    hs.message = condition.message

    if condition.status == "True" then
      hs.status = "Healthy"
      return hs
    end

    if condition.reason == "UpdateFailed" then
      hs.status = "Degraded"
      return hs
    end

  end
end

return hs
