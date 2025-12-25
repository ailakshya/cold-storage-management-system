-- Cold Storage Management System - Complete Database Schema
-- Generated from VIP production database on 2025-12-26
-- This migration is idempotent - safe to run multiple times
-- Auto-runs on application startup via embedded migrations

CREATE OR REPLACE FUNCTION public.update_updated_at_column() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;

RETURN NEW;

END;

$$;

CREATE TABLE IF NOT EXISTS public.admin_action_logs (
    id integer NOT NULL,
    admin_user_id integer NOT NULL,
    action_type character varying(50) NOT NULL,
    target_type character varying(50) NOT NULL,
    target_id integer,
    description text NOT NULL,
    old_value text,
    new_value text,
    ip_address character varying(45),
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.admin_action_logs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.admin_action_logs_id_seq OWNED BY public.admin_action_logs.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.alert_thresholds (
    id integer NOT NULL,
    metric_name character varying(100) NOT NULL,
    warning_threshold numeric(15,4),
    critical_threshold numeric(15,4),
    enabled boolean DEFAULT true,
    cooldown_minutes integer DEFAULT 5,
    description text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.alert_thresholds_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.alert_thresholds_id_seq OWNED BY public.alert_thresholds.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.api_request_logs (
    id bigint NOT NULL,
    request_id uuid DEFAULT gen_random_uuid(),
    method character varying(10) NOT NULL,
    path character varying(500) NOT NULL,
    status_code integer NOT NULL,
    duration_ms numeric(10,2) NOT NULL,
    request_size integer DEFAULT 0,
    response_size integer DEFAULT 0,
    user_id integer,
    ip_address character varying(45),
    user_agent text,
    error_message text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.api_request_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.api_request_logs_id_seq OWNED BY public.api_request_logs.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.archived_api_logs (
    id integer NOT NULL,
    season_name character varying(100),
    "timestamp" timestamp without time zone,
    method character varying(10),
    path character varying(255),
    data jsonb
);

CREATE SEQUENCE IF NOT EXISTS public.archived_api_logs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.archived_api_logs_id_seq OWNED BY public.archived_api_logs.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.archived_entries (
    id integer NOT NULL,
    season_name character varying(100),
    original_id integer,
    customer_id integer,
    truck_number character varying(50),
    item_type character varying(100),
    expected_quantity integer,
    created_at timestamp without time zone,
    data jsonb
);

CREATE SEQUENCE IF NOT EXISTS public.archived_entries_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.archived_entries_id_seq OWNED BY public.archived_entries.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.archived_gate_pass_pickups (
    id integer NOT NULL,
    season_name character varying(100),
    original_id integer,
    gate_pass_id integer,
    quantity integer,
    picked_at timestamp without time zone,
    data jsonb
);

CREATE SEQUENCE IF NOT EXISTS public.archived_gate_pass_pickups_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.archived_gate_pass_pickups_id_seq OWNED BY public.archived_gate_pass_pickups.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.archived_gate_passes (
    id integer NOT NULL,
    season_name character varying(100),
    original_id integer,
    entry_id integer,
    customer_id integer,
    status character varying(50),
    quantity integer,
    created_at timestamp without time zone,
    data jsonb
);

CREATE SEQUENCE IF NOT EXISTS public.archived_gate_passes_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.archived_gate_passes_id_seq OWNED BY public.archived_gate_passes.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.archived_invoice_items (
    id integer NOT NULL,
    season_name character varying(100),
    original_id integer,
    invoice_id integer,
    description text,
    amount numeric(12,2),
    data jsonb
);

CREATE SEQUENCE IF NOT EXISTS public.archived_invoice_items_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.archived_invoice_items_id_seq OWNED BY public.archived_invoice_items.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.archived_invoices (
    id integer NOT NULL,
    season_name character varying(100),
    original_id integer,
    customer_id integer,
    invoice_number character varying(50),
    total_amount numeric(12,2),
    status character varying(50),
    created_at timestamp without time zone,
    data jsonb
);

CREATE SEQUENCE IF NOT EXISTS public.archived_invoices_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.archived_invoices_id_seq OWNED BY public.archived_invoices.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.archived_node_metrics (
    id integer NOT NULL,
    season_name character varying(100),
    "timestamp" timestamp without time zone,
    node_name character varying(100),
    data jsonb
);

CREATE SEQUENCE IF NOT EXISTS public.archived_node_metrics_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.archived_node_metrics_id_seq OWNED BY public.archived_node_metrics.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.archived_rent_payments (
    id integer NOT NULL,
    season_name character varying(100),
    original_id integer,
    customer_id integer,
    amount numeric(12,2),
    payment_date date,
    data jsonb
);

CREATE SEQUENCE IF NOT EXISTS public.archived_rent_payments_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.archived_rent_payments_id_seq OWNED BY public.archived_rent_payments.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.archived_room_entries (
    id integer NOT NULL,
    season_name character varying(100),
    original_id integer,
    entry_id integer,
    room_number integer,
    quantity integer,
    created_at timestamp without time zone,
    data jsonb
);

CREATE SEQUENCE IF NOT EXISTS public.archived_room_entries_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.archived_room_entries_id_seq OWNED BY public.archived_room_entries.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.archived_seasons (
    id integer NOT NULL,
    season_name character varying(100) NOT NULL,
    archived_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.archived_seasons_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.archived_seasons_id_seq OWNED BY public.archived_seasons.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.cluster_nodes (
    id integer NOT NULL,
    ip_address character varying(45) NOT NULL,
    hostname character varying(100),
    role character varying(20) DEFAULT 'worker'::character varying,
    status character varying(30) DEFAULT 'pending'::character varying,
    ssh_user character varying(50) DEFAULT 'root'::character varying,
    ssh_port integer DEFAULT 22,
    ssh_key_id integer,
    k3s_version character varying(20),
    os_info character varying(100),
    cpu_cores integer,
    memory_mb integer,
    disk_gb integer,
    last_seen_at timestamp without time zone,
    provisioned_at timestamp without time zone,
    error_message text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.cluster_nodes_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.cluster_nodes_id_seq OWNED BY public.cluster_nodes.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.customer_otps (
    id integer NOT NULL,
    phone character varying(15) NOT NULL,
    otp_code character varying(6) NOT NULL,
    ip_address character varying(50),
    created_at timestamp without time zone DEFAULT now(),
    expires_at timestamp without time zone NOT NULL,
    verified boolean DEFAULT false,
    attempts integer DEFAULT 0,
    CONSTRAINT chk_otp_code_length CHECK ((length((otp_code)::text) = 6))
);

CREATE SEQUENCE IF NOT EXISTS public.customer_otps_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.customer_otps_id_seq OWNED BY public.customer_otps.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.customers (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    phone character varying(15) NOT NULL,
    village character varying(100),
    address text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    so character varying(100)
);

CREATE SEQUENCE IF NOT EXISTS public.customers_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.customers_id_seq OWNED BY public.customers.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.deployment_config (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    image_repo character varying(255) NOT NULL,
    current_version character varying(50),
    deployment_name character varying(100),
    namespace character varying(100) DEFAULT 'default'::character varying,
    replicas integer DEFAULT 2,
    build_command text,
    build_context character varying(255),
    docker_file character varying(255) DEFAULT 'Dockerfile'::character varying,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.deployment_config_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.deployment_config_id_seq OWNED BY public.deployment_config.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.deployment_history (
    id integer NOT NULL,
    deployment_id integer,
    version character varying(50) NOT NULL,
    previous_version character varying(50),
    deployed_by integer,
    status character varying(30) DEFAULT 'pending'::character varying,
    build_output text,
    deploy_output text,
    error_message text,
    started_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    completed_at timestamp without time zone
);

CREATE SEQUENCE IF NOT EXISTS public.deployment_history_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.deployment_history_id_seq OWNED BY public.deployment_history.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.employees (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    phone character varying(15) NOT NULL,
    village character varying(100),
    address text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.employees_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.employees_id_seq OWNED BY public.employees.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.entries (
    id integer NOT NULL,
    customer_id integer,
    phone character varying(15) NOT NULL,
    name character varying(100) NOT NULL,
    village character varying(100),
    expected_quantity integer NOT NULL,
    thock_category character varying(20) NOT NULL,
    thock_number character varying(100),
    created_by_user_id integer,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    so character varying(100) DEFAULT ''::character varying NOT NULL,
    remark text,
    CONSTRAINT entries_truck_category_check CHECK (((thock_category)::text = ANY (ARRAY[('seed'::character varying)::text, ('sell'::character varying)::text])))
);

CREATE SEQUENCE IF NOT EXISTS public.entries_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.entries_id_seq OWNED BY public.entries.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.entry_edit_logs (
    id integer NOT NULL,
    entry_id integer NOT NULL,
    edited_by_user_id integer NOT NULL,
    old_name character varying(255),
    new_name character varying(255),
    old_phone character varying(20),
    new_phone character varying(20),
    old_village character varying(255),
    new_village character varying(255),
    old_so character varying(255),
    new_so character varying(255),
    old_expected_quantity integer,
    new_expected_quantity integer,
    old_thock_category character varying(20),
    new_thock_category character varying(20),
    old_remark text,
    new_remark text,
    edited_at timestamp with time zone DEFAULT now()
);

CREATE SEQUENCE IF NOT EXISTS public.entry_edit_logs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.entry_edit_logs_id_seq OWNED BY public.entry_edit_logs.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.entry_events (
    id integer NOT NULL,
    entry_id integer,
    event_type character varying(50) NOT NULL,
    status character varying(50) NOT NULL,
    notes text,
    created_by_user_id integer,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.entry_events_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.entry_events_id_seq OWNED BY public.entry_events.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.external_databases (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    ip_address character varying(45) NOT NULL,
    port integer DEFAULT 5432,
    db_name character varying(100) DEFAULT 'cold_db'::character varying,
    db_user character varying(100) DEFAULT 'postgres'::character varying,
    role character varying(30) DEFAULT 'replica'::character varying,
    status character varying(30) DEFAULT 'unknown'::character varying,
    replication_source_id integer,
    ssh_user character varying(50) DEFAULT 'root'::character varying,
    ssh_port integer DEFAULT 22,
    pg_version character varying(20),
    connection_count integer,
    replication_lag_seconds double precision,
    disk_usage_percent integer,
    last_backup_at timestamp without time zone,
    last_checked_at timestamp without time zone,
    error_message text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.external_databases_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.external_databases_id_seq OWNED BY public.external_databases.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.gate_pass_pickup_gatars (
    id integer NOT NULL,
    pickup_id integer NOT NULL,
    gatar_no integer NOT NULL,
    quantity integer NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT gate_pass_pickup_gatars_quantity_check CHECK ((quantity > 0))
);

CREATE SEQUENCE IF NOT EXISTS public.gate_pass_pickup_gatars_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.gate_pass_pickup_gatars_id_seq OWNED BY public.gate_pass_pickup_gatars.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.gate_pass_pickups (
    id integer NOT NULL,
    gate_pass_id integer NOT NULL,
    pickup_quantity integer NOT NULL,
    picked_up_by_user_id integer NOT NULL,
    pickup_time timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    room_no character varying(10),
    floor character varying(10),
    remarks text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.gate_pass_pickups_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.gate_pass_pickups_id_seq OWNED BY public.gate_pass_pickups.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.gate_passes (
    id integer NOT NULL,
    customer_id integer NOT NULL,
    thock_number character varying(20) NOT NULL,
    entry_id integer,
    requested_quantity integer NOT NULL,
    approved_quantity integer,
    gate_no character varying(50),
    status character varying(20) DEFAULT 'pending'::character varying,
    payment_verified boolean DEFAULT false,
    payment_amount numeric(10,2),
    issued_by_user_id integer,
    approved_by_user_id integer,
    issued_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    completed_at timestamp without time zone,
    remarks text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    created_by_customer_id integer,
    request_source character varying(20) DEFAULT 'employee'::character varying,
    expires_at timestamp without time zone,
    total_picked_up integer DEFAULT 0,
    approval_expires_at timestamp without time zone,
    final_approved_quantity integer,
    CONSTRAINT gate_passes_request_source_check CHECK (((request_source)::text = ANY (ARRAY[('employee'::character varying)::text, ('customer_portal'::character varying)::text]))),
    CONSTRAINT gate_passes_status_check CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('approved'::character varying)::text, ('completed'::character varying)::text, ('expired'::character varying)::text, ('rejected'::character varying)::text, ('partially_completed'::character varying)::text])))
);

CREATE SEQUENCE IF NOT EXISTS public.gate_passes_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.gate_passes_id_seq OWNED BY public.gate_passes.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.guard_entries (
    id integer NOT NULL,
    customer_name character varying(100) NOT NULL,
    village character varying(100) NOT NULL,
    mobile character varying(15) NOT NULL,
    arrival_time timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    category character varying(10),
    remarks text,
    status character varying(20) DEFAULT 'pending'::character varying,
    created_by_user_id integer NOT NULL,
    processed_by_user_id integer,
    processed_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    driver_no character varying(15),
    quantity integer DEFAULT 0,
    token_number integer,
    so character varying(100),
    seed_quantity integer DEFAULT 0,
    sell_quantity integer DEFAULT 0,
    seed_processed boolean DEFAULT false,
    sell_processed boolean DEFAULT false,
    seed_processed_by integer,
    sell_processed_by integer,
    seed_processed_at timestamp without time zone,
    sell_processed_at timestamp without time zone,
    seed_qty_1 integer DEFAULT 0,
    seed_qty_2 integer DEFAULT 0,
    seed_qty_3 integer DEFAULT 0,
    seed_qty_4 integer DEFAULT 0,
    sell_qty_1 integer DEFAULT 0,
    sell_qty_2 integer DEFAULT 0,
    sell_qty_3 integer DEFAULT 0,
    sell_qty_4 integer DEFAULT 0,
    CONSTRAINT guard_entries_category_check CHECK (((category)::text = ANY (ARRAY[('seed'::character varying)::text, ('sell'::character varying)::text, ('both'::character varying)::text]))),
    CONSTRAINT guard_entries_status_check CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('processed'::character varying)::text])))
);

CREATE SEQUENCE IF NOT EXISTS public.guard_entries_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.guard_entries_id_seq OWNED BY public.guard_entries.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE SEQUENCE IF NOT EXISTS public.guard_entry_token_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.infra_action_logs (
    id integer NOT NULL,
    user_id integer,
    action character varying(100) NOT NULL,
    target_type character varying(50),
    target_id character varying(100),
    details jsonb,
    status character varying(20) DEFAULT 'success'::character varying,
    error_message text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.infra_action_logs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.infra_action_logs_id_seq OWNED BY public.infra_action_logs.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.infra_config (
    id integer NOT NULL,
    key character varying(100) NOT NULL,
    value text NOT NULL,
    description text,
    is_secret boolean DEFAULT false,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.infra_config_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.infra_config_id_seq OWNED BY public.infra_config.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.invoice_items (
    id integer NOT NULL,
    invoice_id integer,
    entry_id integer,
    thock_number character varying(100),
    quantity integer,
    rate numeric(10,2),
    amount numeric(10,2),
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.invoice_items_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.invoice_items_id_seq OWNED BY public.invoice_items.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE SEQUENCE IF NOT EXISTS public.invoice_number_sequence
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.invoices (
    id integer NOT NULL,
    invoice_number character varying(50) NOT NULL,
    customer_id integer,
    employee_id integer,
    total_amount numeric(10,2) DEFAULT 0 NOT NULL,
    items_count integer DEFAULT 0,
    notes text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.invoices_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.invoices_id_seq OWNED BY public.invoices.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.login_logs (
    id integer NOT NULL,
    user_id integer NOT NULL,
    login_time timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    logout_time timestamp without time zone,
    ip_address character varying(45),
    user_agent text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.login_logs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.login_logs_id_seq OWNED BY public.login_logs.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.monitoring_alerts (
    id integer NOT NULL,
    alert_type character varying(50) NOT NULL,
    severity character varying(20) NOT NULL,
    source character varying(100) NOT NULL,
    title character varying(200) NOT NULL,
    message text NOT NULL,
    metric_value numeric(15,4),
    threshold_value numeric(15,4),
    node_name character varying(100),
    acknowledged boolean DEFAULT false,
    acknowledged_by integer,
    acknowledged_at timestamp without time zone,
    resolved boolean DEFAULT false,
    resolved_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT monitoring_alerts_severity_check CHECK (((severity)::text = ANY (ARRAY[('info'::character varying)::text, ('warning'::character varying)::text, ('critical'::character varying)::text])))
);

CREATE SEQUENCE IF NOT EXISTS public.monitoring_alerts_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.monitoring_alerts_id_seq OWNED BY public.monitoring_alerts.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.node_metrics (
    id bigint NOT NULL,
    node_name character varying(100) NOT NULL,
    node_ip character varying(45) NOT NULL,
    cpu_percent numeric(5,2),
    cpu_cores integer,
    memory_used_bytes bigint,
    memory_total_bytes bigint,
    memory_percent numeric(5,2),
    disk_used_bytes bigint,
    disk_total_bytes bigint,
    disk_percent numeric(5,2),
    network_rx_bytes bigint,
    network_tx_bytes bigint,
    pod_count integer,
    node_status character varying(20) DEFAULT 'Ready'::character varying,
    node_role character varying(50),
    load_average_1m numeric(6,2),
    load_average_5m numeric(6,2),
    load_average_15m numeric(6,2),
    collected_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.node_metrics_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.node_metrics_id_seq OWNED BY public.node_metrics.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.node_provision_logs (
    id integer NOT NULL,
    node_id integer,
    step character varying(100) NOT NULL,
    status character varying(20) NOT NULL,
    message text,
    output text,
    started_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    finished_at timestamp without time zone
);

CREATE SEQUENCE IF NOT EXISTS public.node_provision_logs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.node_provision_logs_id_seq OWNED BY public.node_provision_logs.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.receipt_archives (
    id integer NOT NULL,
    receipt_type character varying(50) NOT NULL,
    receipt_number character varying(100) NOT NULL,
    source_id integer NOT NULL,
    customer_phone character varying(20),
    customer_name character varying(255),
    file_path character varying(500) NOT NULL,
    file_name character varying(255) NOT NULL,
    file_size_bytes bigint,
    generated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    generated_by_user_id integer,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.receipt_archives_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.receipt_archives_id_seq OWNED BY public.receipt_archives.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE SEQUENCE IF NOT EXISTS public.receipt_number_sequence
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.rent_payments (
    id integer NOT NULL,
    entry_id integer NOT NULL,
    customer_name character varying(100) NOT NULL,
    customer_phone character varying(15) NOT NULL,
    total_rent numeric(10,2) NOT NULL,
    amount_paid numeric(10,2) NOT NULL,
    balance numeric(10,2) NOT NULL,
    payment_date timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    processed_by_user_id integer,
    notes text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    receipt_number character varying(50) NOT NULL
);

CREATE SEQUENCE IF NOT EXISTS public.rent_payments_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.rent_payments_id_seq OWNED BY public.rent_payments.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.room_entries (
    id integer NOT NULL,
    entry_id integer,
    thock_number character varying(100) NOT NULL,
    room_no character varying(10) NOT NULL,
    floor character varying(10) NOT NULL,
    gate_no character varying(50) NOT NULL,
    remark character varying(100),
    quantity integer NOT NULL,
    created_by_user_id integer,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    quantity_breakdown character varying(255)
);

CREATE SEQUENCE IF NOT EXISTS public.room_entries_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.room_entries_id_seq OWNED BY public.room_entries.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.room_entry_edit_logs (
    id integer NOT NULL,
    room_entry_id integer NOT NULL,
    edited_by_user_id integer NOT NULL,
    old_room_no character varying(10),
    new_room_no character varying(10),
    old_floor character varying(10),
    new_floor character varying(10),
    old_gate_no character varying(50),
    new_gate_no character varying(50),
    old_quantity integer,
    new_quantity integer,
    old_remark text,
    new_remark text,
    edited_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.room_entry_edit_logs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.room_entry_edit_logs_id_seq OWNED BY public.room_entry_edit_logs.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.season_requests (
    id integer NOT NULL,
    status character varying(50) DEFAULT 'pending'::character varying NOT NULL,
    initiated_by_user_id integer NOT NULL,
    initiated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    approved_by_user_id integer,
    approved_at timestamp without time zone,
    archive_location text,
    records_archived jsonb,
    error_message text,
    season_name character varying(100),
    notes text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_different_admins CHECK (((approved_by_user_id IS NULL) OR (approved_by_user_id <> initiated_by_user_id) OR ((approved_by_user_id = 2) AND (initiated_by_user_id = 2))))
);

CREATE SEQUENCE IF NOT EXISTS public.season_requests_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.season_requests_id_seq OWNED BY public.season_requests.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE SEQUENCE IF NOT EXISTS public.seed_entry_sequence
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE SEQUENCE IF NOT EXISTS public.sell_entry_sequence
    START WITH 1501
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.ssh_keys (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    public_key text NOT NULL,
    private_key_path character varying(255),
    fingerprint character varying(100),
    is_default boolean DEFAULT false,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.ssh_keys_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.ssh_keys_id_seq OWNED BY public.ssh_keys.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.system_settings (
    id integer NOT NULL,
    setting_key character varying(100) NOT NULL,
    setting_value text NOT NULL,
    description text,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_by_user_id integer
);

CREATE SEQUENCE IF NOT EXISTS public.system_settings_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.system_settings_id_seq OWNED BY public.system_settings.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.token_colors (
    id integer NOT NULL,
    color_date date NOT NULL,
    color character varying(20) NOT NULL,
    set_by_user_id integer,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS public.token_colors_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.token_colors_id_seq OWNED BY public.token_colors.id;
EXCEPTION WHEN others THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS public.users (
    id integer NOT NULL,
    name text NOT NULL,
    email text NOT NULL,
    password_hash text DEFAULT ''::text NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    role character varying(20) DEFAULT 'employee'::character varying,
    phone character varying(15),
    village character varying(100),
    has_accountant_access boolean DEFAULT false,
    is_active boolean DEFAULT true NOT NULL
);

CREATE SEQUENCE IF NOT EXISTS public.users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

DO $$
BEGIN
    ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.admin_action_logs ALTER COLUMN id SET DEFAULT nextval('public.admin_action_logs_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.alert_thresholds ALTER COLUMN id SET DEFAULT nextval('public.alert_thresholds_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.api_request_logs ALTER COLUMN id SET DEFAULT nextval('public.api_request_logs_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_api_logs ALTER COLUMN id SET DEFAULT nextval('public.archived_api_logs_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_entries ALTER COLUMN id SET DEFAULT nextval('public.archived_entries_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_gate_pass_pickups ALTER COLUMN id SET DEFAULT nextval('public.archived_gate_pass_pickups_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_gate_passes ALTER COLUMN id SET DEFAULT nextval('public.archived_gate_passes_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_invoice_items ALTER COLUMN id SET DEFAULT nextval('public.archived_invoice_items_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_invoices ALTER COLUMN id SET DEFAULT nextval('public.archived_invoices_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_node_metrics ALTER COLUMN id SET DEFAULT nextval('public.archived_node_metrics_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_rent_payments ALTER COLUMN id SET DEFAULT nextval('public.archived_rent_payments_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_room_entries ALTER COLUMN id SET DEFAULT nextval('public.archived_room_entries_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_seasons ALTER COLUMN id SET DEFAULT nextval('public.archived_seasons_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.cluster_nodes ALTER COLUMN id SET DEFAULT nextval('public.cluster_nodes_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.customer_otps ALTER COLUMN id SET DEFAULT nextval('public.customer_otps_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.customers ALTER COLUMN id SET DEFAULT nextval('public.customers_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.deployment_config ALTER COLUMN id SET DEFAULT nextval('public.deployment_config_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.deployment_history ALTER COLUMN id SET DEFAULT nextval('public.deployment_history_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.employees ALTER COLUMN id SET DEFAULT nextval('public.employees_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.entries ALTER COLUMN id SET DEFAULT nextval('public.entries_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.entry_edit_logs ALTER COLUMN id SET DEFAULT nextval('public.entry_edit_logs_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.entry_events ALTER COLUMN id SET DEFAULT nextval('public.entry_events_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.external_databases ALTER COLUMN id SET DEFAULT nextval('public.external_databases_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.gate_pass_pickup_gatars ALTER COLUMN id SET DEFAULT nextval('public.gate_pass_pickup_gatars_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.gate_pass_pickups ALTER COLUMN id SET DEFAULT nextval('public.gate_pass_pickups_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.gate_passes ALTER COLUMN id SET DEFAULT nextval('public.gate_passes_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.guard_entries ALTER COLUMN id SET DEFAULT nextval('public.guard_entries_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.infra_action_logs ALTER COLUMN id SET DEFAULT nextval('public.infra_action_logs_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.infra_config ALTER COLUMN id SET DEFAULT nextval('public.infra_config_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.invoice_items ALTER COLUMN id SET DEFAULT nextval('public.invoice_items_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.invoices ALTER COLUMN id SET DEFAULT nextval('public.invoices_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.login_logs ALTER COLUMN id SET DEFAULT nextval('public.login_logs_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.monitoring_alerts ALTER COLUMN id SET DEFAULT nextval('public.monitoring_alerts_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.node_metrics ALTER COLUMN id SET DEFAULT nextval('public.node_metrics_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.node_provision_logs ALTER COLUMN id SET DEFAULT nextval('public.node_provision_logs_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.receipt_archives ALTER COLUMN id SET DEFAULT nextval('public.receipt_archives_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.rent_payments ALTER COLUMN id SET DEFAULT nextval('public.rent_payments_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.room_entries ALTER COLUMN id SET DEFAULT nextval('public.room_entries_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.room_entry_edit_logs ALTER COLUMN id SET DEFAULT nextval('public.room_entry_edit_logs_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.season_requests ALTER COLUMN id SET DEFAULT nextval('public.season_requests_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.ssh_keys ALTER COLUMN id SET DEFAULT nextval('public.ssh_keys_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.system_settings ALTER COLUMN id SET DEFAULT nextval('public.system_settings_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.token_colors ALTER COLUMN id SET DEFAULT nextval('public.token_colors_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);
EXCEPTION WHEN others THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.admin_action_logs
    ADD CONSTRAINT admin_action_logs_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.alert_thresholds
    ADD CONSTRAINT alert_thresholds_metric_name_key UNIQUE (metric_name);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.alert_thresholds
    ADD CONSTRAINT alert_thresholds_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.api_request_logs
    ADD CONSTRAINT api_request_logs_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_api_logs
    ADD CONSTRAINT archived_api_logs_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_entries
    ADD CONSTRAINT archived_entries_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_gate_pass_pickups
    ADD CONSTRAINT archived_gate_pass_pickups_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_gate_passes
    ADD CONSTRAINT archived_gate_passes_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_invoice_items
    ADD CONSTRAINT archived_invoice_items_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_invoices
    ADD CONSTRAINT archived_invoices_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_node_metrics
    ADD CONSTRAINT archived_node_metrics_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_rent_payments
    ADD CONSTRAINT archived_rent_payments_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_room_entries
    ADD CONSTRAINT archived_room_entries_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.archived_seasons
    ADD CONSTRAINT archived_seasons_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.cluster_nodes
    ADD CONSTRAINT cluster_nodes_ip_address_key UNIQUE (ip_address);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.cluster_nodes
    ADD CONSTRAINT cluster_nodes_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.customer_otps
    ADD CONSTRAINT customer_otps_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.customers
    ADD CONSTRAINT customers_phone_key UNIQUE (phone);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.customers
    ADD CONSTRAINT customers_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.deployment_config
    ADD CONSTRAINT deployment_config_name_key UNIQUE (name);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.deployment_config
    ADD CONSTRAINT deployment_config_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.deployment_history
    ADD CONSTRAINT deployment_history_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.employees
    ADD CONSTRAINT employees_phone_key UNIQUE (phone);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.employees
    ADD CONSTRAINT employees_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.entries
    ADD CONSTRAINT entries_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.entries
    ADD CONSTRAINT entries_thock_number_unique UNIQUE (thock_number);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.entry_edit_logs
    ADD CONSTRAINT entry_edit_logs_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.entry_events
    ADD CONSTRAINT entry_events_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.external_databases
    ADD CONSTRAINT external_databases_ip_address_port_key UNIQUE (ip_address, port);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.external_databases
    ADD CONSTRAINT external_databases_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.gate_pass_pickup_gatars
    ADD CONSTRAINT gate_pass_pickup_gatars_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.gate_pass_pickups
    ADD CONSTRAINT gate_pass_pickups_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.gate_passes
    ADD CONSTRAINT gate_passes_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.guard_entries
    ADD CONSTRAINT guard_entries_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.infra_action_logs
    ADD CONSTRAINT infra_action_logs_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.infra_config
    ADD CONSTRAINT infra_config_key_key UNIQUE (key);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.infra_config
    ADD CONSTRAINT infra_config_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.invoice_items
    ADD CONSTRAINT invoice_items_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT invoices_invoice_number_key UNIQUE (invoice_number);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT invoices_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.login_logs
    ADD CONSTRAINT login_logs_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.monitoring_alerts
    ADD CONSTRAINT monitoring_alerts_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.node_metrics
    ADD CONSTRAINT node_metrics_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.node_provision_logs
    ADD CONSTRAINT node_provision_logs_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.receipt_archives
    ADD CONSTRAINT receipt_archives_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.rent_payments
    ADD CONSTRAINT rent_payments_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.rent_payments
    ADD CONSTRAINT rent_payments_receipt_number_key UNIQUE (receipt_number);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.room_entries
    ADD CONSTRAINT room_entries_entry_room_unique UNIQUE (entry_id, room_no);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.room_entries
    ADD CONSTRAINT room_entries_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.room_entry_edit_logs
    ADD CONSTRAINT room_entry_edit_logs_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.season_requests
    ADD CONSTRAINT season_requests_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.ssh_keys
    ADD CONSTRAINT ssh_keys_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.system_settings
    ADD CONSTRAINT system_settings_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.system_settings
    ADD CONSTRAINT system_settings_setting_key_key UNIQUE (setting_key);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.token_colors
    ADD CONSTRAINT token_colors_color_date_key UNIQUE (color_date);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.token_colors
    ADD CONSTRAINT token_colors_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.receipt_archives
    ADD CONSTRAINT unique_receipt UNIQUE (receipt_type, source_id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_admin_action_logs_action_type ON public.admin_action_logs USING btree (action_type);

CREATE INDEX IF NOT EXISTS idx_admin_action_logs_admin_user_id ON public.admin_action_logs USING btree (admin_user_id);

CREATE INDEX IF NOT EXISTS idx_admin_action_logs_created_at ON public.admin_action_logs USING btree (created_at);

CREATE INDEX IF NOT EXISTS idx_alerts_active ON public.monitoring_alerts USING btree (resolved, acknowledged);

CREATE INDEX IF NOT EXISTS idx_alerts_created_at ON public.monitoring_alerts USING btree (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_alerts_resolved ON public.monitoring_alerts USING btree (resolved);

CREATE INDEX IF NOT EXISTS idx_alerts_severity ON public.monitoring_alerts USING btree (severity);

CREATE INDEX IF NOT EXISTS idx_alerts_type ON public.monitoring_alerts USING btree (alert_type);

CREATE INDEX IF NOT EXISTS idx_api_logs_created_at ON public.api_request_logs USING btree (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_api_logs_method ON public.api_request_logs USING btree (method);

CREATE INDEX IF NOT EXISTS idx_api_logs_path ON public.api_request_logs USING btree (path);

CREATE INDEX IF NOT EXISTS idx_api_logs_path_status ON public.api_request_logs USING btree (path, status_code);

CREATE INDEX IF NOT EXISTS idx_api_logs_status_code ON public.api_request_logs USING btree (status_code);

CREATE INDEX IF NOT EXISTS idx_api_logs_user_id ON public.api_request_logs USING btree (user_id);

CREATE INDEX IF NOT EXISTS idx_cluster_nodes_role ON public.cluster_nodes USING btree (role);

CREATE INDEX IF NOT EXISTS idx_cluster_nodes_status ON public.cluster_nodes USING btree (status);

CREATE INDEX IF NOT EXISTS idx_customer_otps_expires_at ON public.customer_otps USING btree (expires_at);

CREATE INDEX IF NOT EXISTS idx_customer_otps_ip_created ON public.customer_otps USING btree (ip_address, created_at);

CREATE INDEX IF NOT EXISTS idx_customer_otps_phone_created ON public.customer_otps USING btree (phone, created_at);

CREATE INDEX IF NOT EXISTS idx_customers_name ON public.customers USING btree (name);

CREATE INDEX IF NOT EXISTS idx_customers_phone ON public.customers USING btree (phone);

CREATE INDEX IF NOT EXISTS idx_customers_phone_prefix ON public.customers USING btree (phone varchar_pattern_ops);

CREATE INDEX IF NOT EXISTS idx_customers_so ON public.customers USING btree (so);

CREATE INDEX IF NOT EXISTS idx_deployment_history_deployment ON public.deployment_history USING btree (deployment_id);

CREATE INDEX IF NOT EXISTS idx_deployment_history_status ON public.deployment_history USING btree (status);

CREATE INDEX IF NOT EXISTS idx_edit_logs_edited_at ON public.room_entry_edit_logs USING btree (edited_at DESC);

CREATE INDEX IF NOT EXISTS idx_edit_logs_edited_by ON public.room_entry_edit_logs USING btree (edited_by_user_id);

CREATE INDEX IF NOT EXISTS idx_edit_logs_room_entry_id ON public.room_entry_edit_logs USING btree (room_entry_id);

CREATE INDEX IF NOT EXISTS idx_employees_name ON public.employees USING btree (name);

CREATE INDEX IF NOT EXISTS idx_employees_phone ON public.employees USING btree (phone);

CREATE INDEX IF NOT EXISTS idx_entries_category_created ON public.entries USING btree (thock_category, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_entries_created_at ON public.entries USING btree (created_at);

CREATE INDEX IF NOT EXISTS idx_entries_created_at_customer ON public.entries USING btree (created_at DESC, customer_id);

CREATE INDEX IF NOT EXISTS idx_entries_created_by_user ON public.entries USING btree (created_by_user_id);

CREATE INDEX IF NOT EXISTS idx_entries_customer_id ON public.entries USING btree (customer_id);

CREATE INDEX IF NOT EXISTS idx_entries_phone ON public.entries USING btree (phone);

CREATE INDEX IF NOT EXISTS idx_entries_thock_number ON public.entries USING btree (thock_number);

CREATE INDEX IF NOT EXISTS idx_entry_edit_logs_edited_at ON public.entry_edit_logs USING btree (edited_at DESC);

CREATE INDEX IF NOT EXISTS idx_entry_edit_logs_edited_by ON public.entry_edit_logs USING btree (edited_by_user_id);

CREATE INDEX IF NOT EXISTS idx_entry_edit_logs_entry_id ON public.entry_edit_logs USING btree (entry_id);

CREATE INDEX IF NOT EXISTS idx_entry_events_created_at ON public.entry_events USING btree (created_at);

CREATE INDEX IF NOT EXISTS idx_entry_events_entry_id ON public.entry_events USING btree (entry_id);

CREATE INDEX IF NOT EXISTS idx_entry_events_status ON public.entry_events USING btree (status);

CREATE INDEX IF NOT EXISTS idx_external_databases_role ON public.external_databases USING btree (role);

CREATE INDEX IF NOT EXISTS idx_external_databases_status ON public.external_databases USING btree (status);

CREATE INDEX IF NOT EXISTS idx_gate_pass_pickups_gate_pass ON public.gate_pass_pickups USING btree (gate_pass_id, pickup_time DESC);

CREATE INDEX IF NOT EXISTS idx_gate_pass_pickups_gate_pass_id ON public.gate_pass_pickups USING btree (gate_pass_id);

CREATE INDEX IF NOT EXISTS idx_gate_pass_pickups_pickup_time ON public.gate_pass_pickups USING btree (pickup_time);

CREATE INDEX IF NOT EXISTS idx_gate_passes_created_by_customer ON public.gate_passes USING btree (created_by_customer_id) WHERE (created_by_customer_id IS NOT NULL);

CREATE INDEX IF NOT EXISTS idx_gate_passes_customer_id ON public.gate_passes USING btree (customer_id);

CREATE INDEX IF NOT EXISTS idx_gate_passes_entry_id ON public.gate_passes USING btree (entry_id);

CREATE INDEX IF NOT EXISTS idx_gate_passes_expires_at ON public.gate_passes USING btree (expires_at);

CREATE INDEX IF NOT EXISTS idx_gate_passes_issued_at ON public.gate_passes USING btree (issued_at);

CREATE INDEX IF NOT EXISTS idx_gate_passes_request_source ON public.gate_passes USING btree (request_source);

CREATE INDEX IF NOT EXISTS idx_gate_passes_status ON public.gate_passes USING btree (status);

CREATE INDEX IF NOT EXISTS idx_gate_passes_status_updated ON public.gate_passes USING btree (status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_gate_passes_thock_number ON public.gate_passes USING btree (thock_number);

CREATE INDEX IF NOT EXISTS idx_gate_passes_truck_number ON public.gate_passes USING btree (thock_number);

CREATE INDEX IF NOT EXISTS idx_guard_entries_arrival_time ON public.guard_entries USING btree (arrival_time);

CREATE INDEX IF NOT EXISTS idx_guard_entries_created_by ON public.guard_entries USING btree (created_by_user_id);

CREATE INDEX IF NOT EXISTS idx_guard_entries_date ON public.guard_entries USING btree (date(created_at));

CREATE INDEX IF NOT EXISTS idx_guard_entries_driver_no ON public.guard_entries USING btree (driver_no);

CREATE INDEX IF NOT EXISTS idx_guard_entries_mobile ON public.guard_entries USING btree (mobile);

CREATE INDEX IF NOT EXISTS idx_guard_entries_partial ON public.guard_entries USING btree (seed_processed, sell_processed) WHERE ((status)::text = 'pending'::text);

CREATE INDEX IF NOT EXISTS idx_guard_entries_status ON public.guard_entries USING btree (status);

CREATE INDEX IF NOT EXISTS idx_guard_entries_status_date ON public.guard_entries USING btree (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_guard_entries_token ON public.guard_entries USING btree (token_number);

CREATE INDEX IF NOT EXISTS idx_guard_entries_user_created ON public.guard_entries USING btree (created_by_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_infra_action_logs_action ON public.infra_action_logs USING btree (action);

CREATE INDEX IF NOT EXISTS idx_infra_action_logs_user ON public.infra_action_logs USING btree (user_id);

CREATE INDEX IF NOT EXISTS idx_infra_config_key ON public.infra_config USING btree (key);

CREATE INDEX IF NOT EXISTS idx_invoice_items_entry_id ON public.invoice_items USING btree (entry_id);

CREATE INDEX IF NOT EXISTS idx_invoice_items_invoice_id ON public.invoice_items USING btree (invoice_id);

CREATE INDEX IF NOT EXISTS idx_invoices_created_at ON public.invoices USING btree (created_at);

CREATE INDEX IF NOT EXISTS idx_invoices_customer_id ON public.invoices USING btree (customer_id);

CREATE INDEX IF NOT EXISTS idx_invoices_employee_id ON public.invoices USING btree (employee_id);

CREATE INDEX IF NOT EXISTS idx_invoices_invoice_number ON public.invoices USING btree (invoice_number);

CREATE INDEX IF NOT EXISTS idx_login_logs_login_time ON public.login_logs USING btree (login_time);

CREATE INDEX IF NOT EXISTS idx_login_logs_user_id ON public.login_logs USING btree (user_id);

CREATE INDEX IF NOT EXISTS idx_node_metrics_collected_at ON public.node_metrics USING btree (collected_at DESC);

CREATE INDEX IF NOT EXISTS idx_node_metrics_node_ip ON public.node_metrics USING btree (node_ip);

CREATE INDEX IF NOT EXISTS idx_node_metrics_node_time ON public.node_metrics USING btree (node_name, collected_at DESC);

CREATE INDEX IF NOT EXISTS idx_node_provision_logs_node ON public.node_provision_logs USING btree (node_id);

CREATE INDEX IF NOT EXISTS idx_pickup_gatars_pickup_id ON public.gate_pass_pickup_gatars USING btree (pickup_id);

CREATE INDEX IF NOT EXISTS idx_receipt_archives_customer_phone ON public.receipt_archives USING btree (customer_phone);

CREATE INDEX IF NOT EXISTS idx_receipt_archives_generated_at ON public.receipt_archives USING btree (generated_at);

CREATE INDEX IF NOT EXISTS idx_receipt_archives_receipt_number ON public.receipt_archives USING btree (receipt_number);

CREATE INDEX IF NOT EXISTS idx_receipt_archives_type ON public.receipt_archives USING btree (receipt_type);

CREATE INDEX IF NOT EXISTS idx_rent_payments_date ON public.rent_payments USING btree (payment_date);

CREATE INDEX IF NOT EXISTS idx_rent_payments_entry_id ON public.rent_payments USING btree (entry_id);

CREATE INDEX IF NOT EXISTS idx_rent_payments_phone ON public.rent_payments USING btree (customer_phone);

CREATE INDEX IF NOT EXISTS idx_rent_payments_phone_date ON public.rent_payments USING btree (customer_phone, payment_date DESC);

CREATE INDEX IF NOT EXISTS idx_rent_payments_receipt_number ON public.rent_payments USING btree (receipt_number);

CREATE INDEX IF NOT EXISTS idx_room_entries_created_at ON public.room_entries USING btree (created_at);

CREATE INDEX IF NOT EXISTS idx_room_entries_entry_id ON public.room_entries USING btree (entry_id);

CREATE INDEX IF NOT EXISTS idx_room_entries_gate_no ON public.room_entries USING btree (gate_no);

CREATE INDEX IF NOT EXISTS idx_room_entries_room_floor ON public.room_entries USING btree (room_no, floor);

CREATE INDEX IF NOT EXISTS idx_room_entries_thock_number ON public.room_entries USING btree (thock_number);

CREATE INDEX IF NOT EXISTS idx_room_entry_edit_logs_edited_by ON public.room_entry_edit_logs USING btree (edited_by_user_id);

CREATE INDEX IF NOT EXISTS idx_room_entry_edit_logs_room_entry_id ON public.room_entry_edit_logs USING btree (room_entry_id);

CREATE INDEX IF NOT EXISTS idx_season_requests_initiated_at ON public.season_requests USING btree (initiated_at DESC);

CREATE INDEX IF NOT EXISTS idx_season_requests_initiated_by ON public.season_requests USING btree (initiated_by_user_id);

CREATE INDEX IF NOT EXISTS idx_season_requests_status ON public.season_requests USING btree (status);

CREATE INDEX IF NOT EXISTS idx_system_settings_key ON public.system_settings USING btree (setting_key);

CREATE INDEX IF NOT EXISTS idx_token_colors_date ON public.token_colors USING btree (color_date);

CREATE INDEX IF NOT EXISTS idx_users_email ON public.users USING btree (email);

CREATE INDEX IF NOT EXISTS idx_users_phone ON public.users USING btree (phone);

CREATE INDEX IF NOT EXISTS idx_users_role ON public.users USING btree (role);

DROP TRIGGER IF EXISTS update_users_updated_at ON public.users;
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON public.users FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

DO $$
BEGIN
    ALTER TABLE ONLY public.admin_action_logs
    ADD CONSTRAINT admin_action_logs_admin_user_id_fkey FOREIGN KEY (admin_user_id) REFERENCES public.users(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.api_request_logs
    ADD CONSTRAINT api_request_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.deployment_history
    ADD CONSTRAINT deployment_history_deployed_by_fkey FOREIGN KEY (deployed_by) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.deployment_history
    ADD CONSTRAINT deployment_history_deployment_id_fkey FOREIGN KEY (deployment_id) REFERENCES public.deployment_config(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.entries
    ADD CONSTRAINT entries_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customers(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.entry_edit_logs
    ADD CONSTRAINT entry_edit_logs_edited_by_user_id_fkey FOREIGN KEY (edited_by_user_id) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.entry_edit_logs
    ADD CONSTRAINT entry_edit_logs_entry_id_fkey FOREIGN KEY (entry_id) REFERENCES public.entries(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.entry_events
    ADD CONSTRAINT entry_events_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.entry_events
    ADD CONSTRAINT entry_events_entry_id_fkey FOREIGN KEY (entry_id) REFERENCES public.entries(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.external_databases
    ADD CONSTRAINT external_databases_replication_source_id_fkey FOREIGN KEY (replication_source_id) REFERENCES public.external_databases(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.gate_pass_pickup_gatars
    ADD CONSTRAINT gate_pass_pickup_gatars_pickup_id_fkey FOREIGN KEY (pickup_id) REFERENCES public.gate_pass_pickups(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.gate_pass_pickups
    ADD CONSTRAINT gate_pass_pickups_gate_pass_id_fkey FOREIGN KEY (gate_pass_id) REFERENCES public.gate_passes(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.gate_pass_pickups
    ADD CONSTRAINT gate_pass_pickups_picked_up_by_user_id_fkey FOREIGN KEY (picked_up_by_user_id) REFERENCES public.users(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.gate_passes
    ADD CONSTRAINT gate_passes_approved_by_user_id_fkey FOREIGN KEY (approved_by_user_id) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.gate_passes
    ADD CONSTRAINT gate_passes_created_by_customer_id_fkey FOREIGN KEY (created_by_customer_id) REFERENCES public.customers(id) ON DELETE SET NULL;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.gate_passes
    ADD CONSTRAINT gate_passes_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customers(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.gate_passes
    ADD CONSTRAINT gate_passes_entry_id_fkey FOREIGN KEY (entry_id) REFERENCES public.entries(id) ON DELETE SET NULL;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.gate_passes
    ADD CONSTRAINT gate_passes_issued_by_user_id_fkey FOREIGN KEY (issued_by_user_id) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.guard_entries
    ADD CONSTRAINT guard_entries_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.guard_entries
    ADD CONSTRAINT guard_entries_processed_by_user_id_fkey FOREIGN KEY (processed_by_user_id) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.infra_action_logs
    ADD CONSTRAINT infra_action_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.invoice_items
    ADD CONSTRAINT invoice_items_entry_id_fkey FOREIGN KEY (entry_id) REFERENCES public.entries(id) ON DELETE SET NULL;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.invoice_items
    ADD CONSTRAINT invoice_items_invoice_id_fkey FOREIGN KEY (invoice_id) REFERENCES public.invoices(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT invoices_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customers(id) ON DELETE SET NULL;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT invoices_employee_id_fkey FOREIGN KEY (employee_id) REFERENCES public.users(id) ON DELETE SET NULL;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.login_logs
    ADD CONSTRAINT login_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.monitoring_alerts
    ADD CONSTRAINT monitoring_alerts_acknowledged_by_fkey FOREIGN KEY (acknowledged_by) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.node_provision_logs
    ADD CONSTRAINT node_provision_logs_node_id_fkey FOREIGN KEY (node_id) REFERENCES public.cluster_nodes(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.receipt_archives
    ADD CONSTRAINT receipt_archives_generated_by_user_id_fkey FOREIGN KEY (generated_by_user_id) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.rent_payments
    ADD CONSTRAINT rent_payments_entry_id_fkey FOREIGN KEY (entry_id) REFERENCES public.entries(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.rent_payments
    ADD CONSTRAINT rent_payments_processed_by_user_id_fkey FOREIGN KEY (processed_by_user_id) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.room_entries
    ADD CONSTRAINT room_entries_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.room_entries
    ADD CONSTRAINT room_entries_entry_id_fkey FOREIGN KEY (entry_id) REFERENCES public.entries(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.room_entry_edit_logs
    ADD CONSTRAINT room_entry_edit_logs_edited_by_user_id_fkey FOREIGN KEY (edited_by_user_id) REFERENCES public.users(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.room_entry_edit_logs
    ADD CONSTRAINT room_entry_edit_logs_room_entry_id_fkey FOREIGN KEY (room_entry_id) REFERENCES public.room_entries(id) ON DELETE CASCADE;
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.season_requests
    ADD CONSTRAINT season_requests_approved_by_user_id_fkey FOREIGN KEY (approved_by_user_id) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.season_requests
    ADD CONSTRAINT season_requests_initiated_by_user_id_fkey FOREIGN KEY (initiated_by_user_id) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.system_settings
    ADD CONSTRAINT system_settings_updated_by_user_id_fkey FOREIGN KEY (updated_by_user_id) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE ONLY public.token_colors
    ADD CONSTRAINT token_colors_set_by_user_id_fkey FOREIGN KEY (set_by_user_id) REFERENCES public.users(id);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;


-- Grant permissions to cold_user for disaster recovery restore
-- This allows R2 backup restore to work when running as cold_user
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO cold_user;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO cold_user;
GRANT USAGE ON SCHEMA public TO cold_user;

-- Also grant default privileges for any future tables
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON TABLES TO cold_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON SEQUENCES TO cold_user;
