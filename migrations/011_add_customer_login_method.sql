-- Add customer login method setting (otp or thock)
INSERT INTO system_settings (setting_key, setting_value, description, updated_by_user_id)
VALUES
    ('customer_login_method', 'otp', 'Customer portal login method: otp or thock', NULL)
ON CONFLICT (setting_key) DO NOTHING;
