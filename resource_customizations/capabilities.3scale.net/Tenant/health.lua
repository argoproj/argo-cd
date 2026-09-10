local hs = {}

if obj ~= nil and obj.status ~= nil then
  local tenant_id = obj.status.tenantId
  local admin_id = obj.status.adminId
  local has_tenant_id = (tenant_id ~= nil and type(tenant_id) == "number" and tenant_id > 0)
  local has_admin_id = (admin_id ~= nil and type(admin_id) == "number" and admin_id > 0)

  local is_ready = false
  if obj.status.conditions ~= nil then
    for _, condition in ipairs(obj.status.conditions) do
      if condition.type == "Ready" and condition.status == "True" then
        is_ready = true
        break
      end
    end
  end

  if is_ready and has_tenant_id and has_admin_id then
    hs.status = "Healthy"
    hs.message = "3scale Tenant is ready"
    return hs
  end

  if obj.status.conditions ~= nil then
    for _, condition in ipairs(obj.status.conditions) do
      if condition.type == "Ready" and condition.status ~= "True" then
        hs.status = "Degraded"
        hs.message = condition.message or "3scale Tenant is degraded"
        return hs
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for 3scale Tenant status..."
return hs
