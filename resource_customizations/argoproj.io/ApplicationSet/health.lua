local hs = {}

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in pairs(obj.status.conditions) do
      if condition.type == "ErrorOccurred" and condition.status == "True" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      end
      if condition.type == "ResourcesUpToDate" and condition.status == "True" then
        hs.status = "Healthy"
        hs.message = condition.message
        return hs
      end
    end
  end
end

-- Conditions were introduced in ApplicationSet v0.3. To give v0.2 users a good experience, we default to "Healthy".
-- Once v0.3 is more generally adopted, we'll default to "Progressing" instead.
hs.status = "Healthy"
hs.message = ""
return hs
