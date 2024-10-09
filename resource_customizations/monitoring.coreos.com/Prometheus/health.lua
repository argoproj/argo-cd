local hs={ status = "Progressing", message = "Waiting for initialization" }

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do

      if condition.type == "Available" and condition.status ~= "True" then
        if condition.reason == "SomePodsNotReady" then
          hs.status = "Progressing"
        else
          hs.status = "Degraded"
        end
        hs.message = condition.message or condition.reason
      end
      if condition.type == "Available" and condition.status == "True" then
        hs.status = "Healthy"
        hs.message = "All instances are available"
      end
    end
  end
end

return hs