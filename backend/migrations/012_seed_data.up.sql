-- Demo user: demo@floq.app / demo123
-- bcrypt hash of "demo123" with cost 10
INSERT INTO users (id, email, password_hash, full_name, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'demo@floq.app',
    '$2a$10$TSmcktowbL3suHj/M0Cpvuul4S/GF71T6YTVDJuYhl6rG90tHU922',
    'Demo User',
    NOW(),
    NOW()
) ON CONFLICT (email) DO NOTHING;

-- Sample prospects for demo user
INSERT INTO prospects (id, user_id, name, company, title, email, phone, telegram_username, industry, source, status, verify_status, verify_details, created_at, updated_at)
VALUES
    ('10000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'Антон Фёдоров', 'TechSolutions', 'Head of Sales', 'a.fedorov@techsol.ru', '+7 916 123-45-67', '@afedorov', 'IT Services', 'manual', 'new', 'not_checked', '{}', NOW(), NOW()),
    ('10000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Елена Кузнецова', 'Global Retail', 'Marketing Director', 'e.kuz@globalretail.com', '+7 925 987-65-43', '@ekuznetsova', 'Retail', 'manual', 'new', 'not_checked', '{}', NOW(), NOW()),
    ('10000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', 'Дмитрий Волков', 'FinStream', 'CTO', 'd.volkov@finstream.io', '+7 903 555-12-34', '@dvolkov', 'FinTech', 'manual', 'new', 'not_checked', '{}', NOW(), NOW()),
    ('10000000-0000-0000-0000-000000000004', '00000000-0000-0000-0000-000000000001', 'Дарья Фролова', 'EcoLife', 'Founder', 'd.frolova@ecolife.ru', '', '', 'Sustainability', 'manual', 'new', 'not_checked', '{}', NOW(), NOW()),
    ('10000000-0000-0000-0000-000000000005', '00000000-0000-0000-0000-000000000001', 'Иван Петров', 'RetailPro', 'Procurement Lead', 'i.petrov@retailpro.com', '+7 912 333-22-11', '', 'Retail', 'manual', 'new', 'not_checked', '{}', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;
