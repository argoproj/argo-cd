hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if (condition.type == "Succeeded" and condition.status == "False") or
         (condition.type == "Failed" and condition.status == "True") then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      end
    end
    hs.status = "Healthy"
    hs.message = "Ready to use"
    return hs
  end
end
hs.status = "Progressing"
hs.message = "Waiting for status"
return hs
