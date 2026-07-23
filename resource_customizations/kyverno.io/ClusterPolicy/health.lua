local hs = {}

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Ready" and condition.status == "True" and condition.reason == "Succeeded" and condition.message == "Ready" then
      hs.status = "Healthy"
      hs.message = "ClusterPolicy is ready"
      return hs
    end
    if condition.type == "Ready" and condition.status == "False" then
      hs.status = "Degraded"
      hs.message = condition.message
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for ClusterPolicy to be ready"
return hs
