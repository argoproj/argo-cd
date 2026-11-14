local hs = {
  status = "Progressing",
  message = "Waiting for the realm import to start",
}

if obj.status == nil or obj.status.conditions == nil then
  return hs
end

-- Keycloak 21 and earlier write the condition status as a boolean.
local function isTrue(condition)
  return condition ~= nil and (condition.status == "True" or condition.status == true)
end

-- The operator also sets HasErrors while it waits for the Keycloak StatefulSet to be created and become ready.
-- From 23.0.0 to 26.3.3 the second message is "Deployment not yet ready, waiting for it to be ready".
-- "Keycloak Deployment not found" is the message of Keycloak 21 and earlier.
-- https://github.com/keycloak/keycloak/blob/26.8.1/operator/src/main/java/org/keycloak/operator/controllers/KeycloakRealmImportController.java
local progressingMessages = {
  "No existing Deployment found",
  "Deployment not yet ready",
  "Keycloak Deployment not found",
}

local done
local started
local hasErrors
for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "Done" then
    done = condition
  elseif condition.type == "Started" then
    started = condition
  elseif condition.type == "HasErrors" then
    hasErrors = condition
  end
end

-- The Job runs "kc.sh import --override=false", which also succeeds when the realm already exists and the import
-- is skipped, so the message doesn't claim that the realm was imported.
if isTrue(done) then
  hs.status = "Healthy"
  hs.message = "Import Job completed"
  return hs
end

if isTrue(hasErrors) then
  local message = hasErrors.message or ""
  hs.message = message
  -- "Import Job failed" is set after the first failed pod while the Job may still retry, and stays the same once the
  -- retries are used up. The Job is owned by the realm import, and its own health turns Degraded at that point.
  -- It is matched exactly, because when the reconciliation throws, the message contains the exception's text.
  if message == "Import Job failed" then
    return hs
  end
  for _, progressing in ipairs(progressingMessages) do
    if string.find(message, progressing, 1, true) ~= nil then
      return hs
    end
  end
  hs.status = "Degraded"
  return hs
end

if isTrue(started) and started.message ~= nil and started.message ~= "" then
  hs.message = started.message
end
return hs
