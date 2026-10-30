hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Ready" then
        if condition.status == "True" then
          hs.status = "Healthy"
          hs.message = condition.message
          return hs
        else
          if condition.reason == "Invalid" then
            hs.status = "Degraded"
            hs.message = condition.message
            return hs
          elseif condition.reason == "MissingResource" then
            hs.status = "Progressing"
            hs.message = condition.message
            return hs
          end
        end
      end
    end
  end
end
hs.status = "Unknown"
hs.message = "Unable to determine the status of the ClusterLogForwarder"
return hs

