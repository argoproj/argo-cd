-- We are setting the suspend flag to false in order to enable
-- the operator
-- see: https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/suspend.md
if obj.spec.suspend ~= nil and obj.spec.suspend then
	obj.spec.suspend = false
end

return obj
