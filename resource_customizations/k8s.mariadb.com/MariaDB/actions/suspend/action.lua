-- We are setting the suspend flag to true in order to disable
-- the operator
-- see: https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/suspend.md
if obj.spec ~= nil then
	obj.spec.suspend = true
end

return obj
