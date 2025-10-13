hs = {}
if obj ~= nil and obj.status ~= nil then
  if type(obj.status.conditions) == "table" then
    for i, condition in ipairs(obj.status.conditions) do
      if condition ~= nil and
         ((condition.type == "Succeeded" and condition.status == "False") or
          (condition.type == "Failed" and condition.status == "True")) then
        hs.status = "Degraded"
        hs.message = condition.message or ""
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