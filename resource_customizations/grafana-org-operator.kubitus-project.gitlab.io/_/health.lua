hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Ready" then
        hs.message = condition.message
        if condition.status == "False" then
          hs.status = "Degraded"
        elseif condition.status == "True" then
          hs.status = "Healthy"
        else
          hs.status = "Progressing"
        end
        return hs
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting ..."
return hs
