hs = {}

local function readyCond(obj)
  if obj.status ~= nil and obj.status.conditions ~= nil then
    for _, condition in ipairs(obj.status.conditions) do
      if condition.type == "Ready" then
        return condition
      end
    end
  end
  return nil
end

local ready = readyCond(obj)

if ready == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for Atlas Operator"
  return hs
end

if ready.status == "True" then
  hs.status = "Healthy"
  hs.message = ready.reason
  return hs
end

if ready.reason == "Reconciling" then
  hs.status = "Progressing"
else
  hs.status = "Degraded"
end

hs.message = ready.reason

return hs

