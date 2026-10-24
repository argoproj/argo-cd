-- AWS ACK: https://aws-controllers-k8s.github.io/docs/
-- ACK conditions have no observedGeneration, so Ready=True from the previous
-- reconcile stays on the object after Argo CD writes spec. Trust that Ready
-- value once ACK.ResourceSynced is at least as new as the argocd manager's
-- spec write. A metadata-only apply bumps the same manager time and is never
-- reconciled, so trust the condition again after a short grace.
local graceSeconds = 30

local function parseTime(value)
  if value == nil or type(value) ~= "string" then
    return nil
  end
  local year, month, day, hour, min, sec = string.match(value, "^(%d+)-(%d+)-(%d+)T(%d+):(%d+):(%d+)")
  if year == nil then
    return nil
  end
  return os.time({
    year = tonumber(year),
    month = tonumber(month),
    day = tonumber(day),
    hour = tonumber(hour),
    min = tonumber(min),
    sec = tonumber(sec),
  })
end

local function specWriteTime()
  if obj.metadata == nil or obj.metadata.managedFields == nil then
    return nil
  end
  local written = nil
  for _, fields in ipairs(obj.metadata.managedFields) do
    local manager = fields.manager
    local ownsSpec = fields.fieldsV1 ~= nil and fields.fieldsV1["f:spec"] ~= nil
    if manager ~= nil and string.sub(manager, 1, 6) == "argocd" and ownsSpec then
      local writtenAt = parseTime(fields.time)
      if writtenAt ~= nil and (written == nil or writtenAt > written) then
        written = writtenAt
      end
    end
  end
  return written
end

local function resourceSyncedTime()
  if obj.status == nil or obj.status.conditions == nil then
    return nil
  end
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "ACK.ResourceSynced" then
      return parseTime(condition.lastTransitionTime)
    end
  end
  return nil
end

local function specObserved()
  local written = specWriteTime()
  if written == nil then
    return true
  end
  local synced = resourceSyncedTime()
  if synced ~= nil and synced >= written then
    return true
  end
  local now = os.date("!*t")
  return os.time(now) - written >= graceSeconds
end

local hs = {}

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "ACK.Terminal" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message
      return hs
    end

    if condition.type == "Ready" then
      if condition.status == "False" then
        hs.status = "Progressing"
        hs.message = condition.message
        return hs
      elseif condition.status == "True" then
        if not specObserved() then
          hs.status = "Progressing"
          hs.message = "Waiting for resource to be ready"
          return hs
        end
        hs.status = "Healthy"
        hs.message = ""
        return hs
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for resource to be ready"
return hs
