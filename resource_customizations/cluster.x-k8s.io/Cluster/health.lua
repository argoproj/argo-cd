hs = {}
if obj.spec.paused then
  hs.status = "Suspended"
  hs.message = "Cluster is paused"
  return hs
elseif obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Ready" then
        if condition.status == "False" then
          if condition.message == nil or condition.message == "" then
            hs.status = "Progressing"
            hs.message = "Waiting for cluster"
          else
            hs.status = "Degraded"
            hs.message = condition.message
          end
          return hs
        else
          hs.status = "Healthy"
          hs.message = "Cluster is running"
          return hs
        end
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for cluster"
return hs