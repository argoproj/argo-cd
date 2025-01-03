local hs = { status = "Progressing", message = "Waiting for initialization" }
found_status = false 

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do

      if condition.type == "Available" then
        found_status = true
        if condition.status ~= "True" then
            if condition.reason == "SomePodsNotReady" then
              hs.status = "Progressing"
            else
              hs.status = "Degraded"
            end
            hs.message = condition.message or condition.reason
        else
            hs.status = "Healthy"
            hs.message = "All instances are available"
        end
      end
    end
  end
end

if not found_status then
    hs = { status = "Unknown", message = "Status is not provided" }
end

return hs
