local hs = {}

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Ready" and condition.status == "True" and condition.reason == "Succeeded" and condition.message == "Ready" then
      hs.status = "Healthy"
      hs.message = "Policy is ready"
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Policy to be ready"
return hs
