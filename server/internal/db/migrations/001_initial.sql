CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    name TEXT NOT NULL,
    secret_key TEXT,
    email_token TEXT,
    email_confirmed BOOLEAN NOT NULL DEFAULT 0,
    last_confirmation_email_sent INTEGER,
    last_personal_data_export INTEGER,
    webhook_urls TEXT, -- JSON map of event type to URL
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE organizations (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT UNIQUE NOT NULL,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE invoices (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    org_id TEXT NOT NULL REFERENCES organizations(id),
    status TEXT NOT NULL, -- pending, processing, processed, failed
    filename TEXT NOT NULL,
    checksum TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    preview_path TEXT,
    ocr_text TEXT,

    -- Extracted fields
    type INTEGER,
    is_invoice BOOLEAN,
    original_invoice_public_id TEXT,
    series_and_number TEXT,
    currency TEXT,
    issue_date INTEGER,
    supply_date INTEGER,
    payment_due_date INTEGER,

    -- Seller details
    seller_name TEXT,
    seller_code TEXT,
    seller_vat TEXT,
    seller_street TEXT,
    seller_city TEXT,
    seller_country TEXT,
    seller_postal_code TEXT,
    seller_email TEXT,
    seller_phone_number TEXT,
    seller_website TEXT,
    seller_individual BOOLEAN,
    seller_banks TEXT,

    -- Buyer details
    buyer_name TEXT,
    buyer_code TEXT,
    buyer_vat TEXT,
    buyer_street TEXT,
    buyer_city TEXT,
    buyer_country TEXT,
    buyer_postal_code TEXT,
    buyer_email TEXT,
    buyer_phone_number TEXT,
    buyer_website TEXT,
    buyer_individual BOOLEAN,
    buyer_banks TEXT,

    -- Totals
    amount_without_vat REAL,
    vat_amount REAL,
    amount_with_vat REAL,
    duplicate_of_id TEXT REFERENCES invoices(id),
    error_message TEXT,

    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE invoice_items (
    id TEXT PRIMARY KEY,
    invoice_id TEXT NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description TEXT,
    quantity REAL,
    unit_price REAL,
    total_price REAL,
    vat_amount REAL,
    vat_rate REAL,
    vat_classifier TEXT,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE companies (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES organizations(id),
    title TEXT NOT NULL,
    code TEXT,
    vat_code TEXT,
    street TEXT,
    city TEXT,
    country TEXT,
    postal_code TEXT,
    email TEXT,
    phone_number TEXT,
    website TEXT,
    individual BOOLEAN,
    banks TEXT,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE vat_classifiers (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES organizations(id),
    country TEXT NOT NULL,
    code TEXT NOT NULL,
    tariff REAL NOT NULL,
    description TEXT,
    example TEXT,
    receiving_rule TEXT,
    issued_rule TEXT,
    active BOOLEAN NOT NULL DEFAULT 1,
    reverse_charge BOOLEAN NOT NULL DEFAULT 0,
    purchase_account TEXT,
    include_in_isaf BOOLEAN NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE export_templates (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES organizations(id),
    type TEXT NOT NULL DEFAULT 'file', -- file, api
    title TEXT NOT NULL,
    description TEXT,
    country TEXT,
    website TEXT,
    active BOOLEAN NOT NULL DEFAULT 1,
    is_system BOOLEAN NOT NULL DEFAULT 0,
    is_favorite BOOLEAN NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE export_template_files (
    id TEXT PRIMARY KEY,
    template_id TEXT NOT NULL REFERENCES export_templates(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX idx_invoices_org_id ON invoices(org_id);
CREATE INDEX idx_invoices_status ON invoices(status);
CREATE INDEX idx_invoices_created_at ON invoices(created_at);
CREATE INDEX idx_invoices_checksum ON invoices(checksum);
CREATE INDEX idx_invoices_user_id ON invoices(user_id);
CREATE INDEX idx_invoices_org_status_created ON invoices(org_id, status, created_at DESC);
CREATE INDEX idx_invoices_org_checksum ON invoices(org_id, checksum);

CREATE INDEX idx_invoice_items_invoice_id ON invoice_items(invoice_id);

CREATE INDEX idx_companies_org_id ON companies(org_id);
CREATE INDEX idx_companies_code ON companies(code);
CREATE INDEX idx_companies_vat_code ON companies(vat_code);
CREATE INDEX idx_companies_org_vat ON companies(org_id, vat_code);
CREATE INDEX idx_companies_org_code ON companies(org_id, code);

CREATE INDEX idx_vat_classifiers_org_id ON vat_classifiers(org_id);
CREATE INDEX idx_vat_classifiers_code ON vat_classifiers(code);
CREATE UNIQUE INDEX IF NOT EXISTS idx_vat_classifiers_org_country_code ON vat_classifiers(org_id, country, code);
CREATE INDEX IF NOT EXISTS idx_vat_classifiers_org_country_active ON vat_classifiers(org_id, country, active);

CREATE INDEX idx_export_templates_org_id ON export_templates(org_id);

CREATE INDEX idx_sessions_token ON sessions(token);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
