hs = {}

-- Check if status exists
if not obj.status then
  hs.status = "Degraded"
  hs.message = "No status available"
  return hs
end

-- Initialize variables for conditions
local available = false
local progressing = false
local availableMessage = ""
local progressingMessage = ""

-- Check conditions
if obj.status.conditions then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Available" then
      available = condition.status == "True"
      availableMessage = condition.message or "Application availability status"
    elseif condition.type == "Progressing" then
      progressing = condition.status == "True"
      progressingMessage = condition.message or "Application progress status"
    end
  end
end

-- Check ready replicas if specified
local readyReplicas = obj.status.readyReplicas or 0
local desiredReplicas = obj.spec.replicas or 1

-- Determine status based on conditions
if not available then
  hs.status = "Degraded"
  hs.message = availableMessage or "Application is not available"
  return hs
end

if readyReplicas < desiredReplicas then
  hs.status = "Progressing"
  hs.message = string.format("Waiting for replicas to be ready (%d/%d)", readyReplicas, desiredReplicas)
  return hs
end

if progressing then
  hs.status = "Progressing"
  hs.message = progressingMessage or "Application is still progressing"
  return hs
end

-- All checks passed
hs.status = "Healthy"
hs.message = string.format("Application is healthy with %d/%d replicas ready", readyReplicas, desiredReplicas)

return hs