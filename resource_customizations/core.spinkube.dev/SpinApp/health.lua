hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Progressing" and condition.status == "False" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      end
      if condition.type == "Available" and condition.status == "True" then
        hs.status = "Healthy"
        hs.message = condition.message
        return hs
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for SpinApp to be available"
return hs