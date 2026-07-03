-- Surface deletion progress while the resource is terminating. You can customize this
-- block, e.g. map known finalizers in obj.metadata.finalizers to clearer messages.
if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil then
  local deletionHs = {}
  deletionHs.status = "Progressing"
  deletionHs.message = "Pending deletion"
  if obj.metadata.finalizers ~= nil and #obj.metadata.finalizers > 0 then
    deletionHs.message = "Pending deletion; blocked by finalizers: " .. table.concat(obj.metadata.finalizers, ", ")
  end
  return deletionHs
end

local hs = {}

if obj.status ~= nil then
  if obj.status.state ~= nil and obj.status.state == "suspended" then
    hs.status = "Suspended"
    hs.message = "3scale Application is suspended"
    return hs
  end

  local application_id = obj.status.adminId
  local has_application_id = (application_id ~= nil and type(application_id) == "number" and application_id > 0)

  if obj.status.conditions ~= nil then
    for _, condition in ipairs(obj.status.conditions) do
      if condition.type == "Ready" then
        if condition.status == "True" then
          hs.status = "Healthy"
          hs.message = "3scale Application is ready"
          return hs
        elseif not has_application_id then
          hs.status = "Degraded"
          hs.message = condition.message or "3scale Application is not ready"
          return hs
        end
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for 3scale Application status..."
return hs
