-- Reference CRD can be found here:
-- https://grafana.github.io/grafana-operator/docs/api/#grafanafolder

function getStatusFromConditions(obj, hs)
  if obj.status ~= nil and obj.status.conditions ~= nil then
      for i, condition in ipairs(obj.status.conditions) do
          if condition.status ~= nil then
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
              hs.status = "Degraded"
              return hs
            end

            if condition.status == "True" then
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
