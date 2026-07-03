local hs = {}
if obj.status ~= nil then
  -- Each message may or may not use these.
  local roleName = obj.status.roleName or "<none>"
  local roleARN = obj.status.roleARN or "<none>"
  local roleID = obj.status.roleID or "<none>"

  if obj.status.state == "Ready" then
    hs.status = "Healthy"
    hs.message = "Role '" .. roleName .. "' exists with ARN '" .. roleARN .. "' and ID '" .. roleID .. "'."
    return hs
  end

  local message = ""
  -- Current non-ready statuses: https://github.com/keikoproj/iam-manager/blob/3aeb2f8ec3005e1c53a057b3b0f79e14a0e5b9cb/api/v1alpha1/iamrole_types.go#L150-L156
  if obj.status.state == "Error" or obj.status.state == "RolesMaxLimitReached" or obj.status.state == "PolicyNotAllowed" or obj.status.state == "RoleNameNotAvailable" then
    hs.status = "Degraded"
    message = "Failed to reconcile the Iamrole "
    if obj.status.retryCount ~= nil and obj.status.retryCount > 0 then
      message = message .. "(retry " .. tostring(obj.status.retryCount) .. ") "
    end
    message = message .. "for role '" .. roleName .. "' with ARN '" .. roleARN .. "' and ID '" .. roleID .. "'."
    if obj.status.errorDescription ~= nil then
      message = message .. " Reconciliation error was: " .. obj.status.errorDescription
    end
    hs.message = message
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Iamrole to be reconciled"
return hs
