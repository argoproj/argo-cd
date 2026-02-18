local hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.status == "True" then
        local msg = condition.reason
        if condition.message ~= nil and condition.message ~= "" then
          msg = condition.reason .. ": " .. condition.message
        end
        if condition.type == "Available" then
          hs.status = "Healthy"
          hs.message = msg
          return hs
        end
        if condition.type == "Degraded" then
          hs.status = "Degraded"
          hs.message = msg
          return hs
        end
        if condition.type == "Progressing" then
          hs.status = "Progressing"
          hs.message = msg
          return hs
        end
        if condition.type == "Ignored" then
          hs.status = "Suspended"
          hs.message = msg
          return hs
        end
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for policy to be applied"
return hs
