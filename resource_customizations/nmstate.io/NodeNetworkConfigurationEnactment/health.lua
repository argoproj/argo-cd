local hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    local failing = false
    local progressing = false

    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Failing" and condition.status == "True" then
        failing = true
      end
      if condition.type == "Progressing" and condition.status == "True" then
        progressing = true
      end
    end

    -- Retrying: Failing=True AND Progressing=True simultaneously
    if failing and progressing then
      hs.status = "Progressing"
      hs.message = "Retrying"
      for i, condition in ipairs(obj.status.conditions) do
        if condition.type == "Failing" and condition.status == "True" then
          local msg = condition.reason
          if condition.message ~= nil and condition.message ~= "" then
            msg = condition.reason .. ": " .. condition.message
          end
          hs.message = msg
        end
      end
      return hs
    end

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
        if condition.type == "Failing" then
          hs.status = "Degraded"
          hs.message = msg
          return hs
        end
        if condition.type == "Progressing" then
          hs.status = "Progressing"
          hs.message = msg
          return hs
        end
        if condition.type == "Pending" then
          hs.status = "Progressing"
          hs.message = msg
          return hs
        end
        if condition.type == "Aborted" then
          hs.status = "Suspended"
          hs.message = msg
          return hs
        end
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for enactment to be applied"
return hs
