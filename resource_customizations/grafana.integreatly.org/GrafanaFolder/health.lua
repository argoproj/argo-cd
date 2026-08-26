-- Reference CRD can be found here:
-- https://grafana.github.io/grafana-operator/docs/api/#grafanafolder

-- Negative-polarity condition types: `status: "False"` means the check
-- passed (no problem), while `status: "True"` indicates a real problem.
-- grafana-operator v5.25.0 introduced FolderUIDMismatch with this
-- polarity (reason: ConsistentUID when there is no mismatch), which must
-- not mark the resource as Degraded.
-- See https://github.com/grafana/grafana-operator/issues/2918
local negativePolarityTypes = {
  FolderUIDMismatch = true,
}

function getStatusFromConditions(obj, hs)
  if obj.status ~= nil and obj.status.conditions ~= nil then
      for i, condition in ipairs(obj.status.conditions) do
          if condition.status ~= nil then
            local isNegativePolarity = condition.type ~= nil and negativePolarityTypes[condition.type] == true

            if hs.message ~= "" then
              hs.message = hs.message .. ", "
            end

            if condition.reason ~= nil then
              hs.message = hs.message .. condition.reason
              if condition.type ~= nil then
                  hs.message = hs.message .. " for " .. condition.type
                if condition.message ~= nil then
                    hs.message = hs.message .. " because " .. condition.message
                end
              end
            end

            if condition.status == "False" then
              if isNegativePolarity then
                -- The check passed; treat like a successful condition.
                hs.status = "Healthy"
              else
                hs.status = "Degraded"
                return hs
              end
            end

            if condition.status == "True" then
              if isNegativePolarity then
                -- A triggered negative-polarity condition is a real problem.
                hs.status = "Degraded"
                return hs
              end

              hs.status = "Healthy"
            end
          end
      end
  end

  return hs
end

local hs = {}
hs.status = "Progressing"
hs.message = ""

hs = getStatusFromConditions(obj, hs)

return hs
