-- Baseline squashed from karots-pos migrations 0001–0074_sale_customer_balance.sql.
-- +goose Up
-- +goose StatementBegin
--
-- PostgreSQL database dump
--


-- Dumped from database version 17.9
-- Dumped by pg_dump version 17.9

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: pg_trgm; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;


--
-- Name: EXTENSION pg_trgm; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pg_trgm IS 'text similarity measurement and index searching based on trigrams';


--
-- Name: cash_movement_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.cash_movement_type AS ENUM (
    'opening',
    'sale',
    'credit_payment',
    'withdrawal',
    'pay_in',
    'refund',
    'closing',
    'expense'
);


--
-- Name: payment_method; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.payment_method AS ENUM (
    'cash',
    'card',
    'online',
    'credit'
);


--
-- Name: purchase_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.purchase_status AS ENUM (
    'draft',
    'received',
    'paid',
    'partial'
);


--
-- Name: sale_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.sale_status AS ENUM (
    'completed',
    'credit',
    'returned',
    'void',
    'partially_returned'
);


--
-- Name: sale_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.sale_type AS ENUM (
    'retail',
    'wholesale',
    'credit'
);


--
-- Name: stock_movement_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.stock_movement_type AS ENUM (
    'purchase',
    'sale',
    'adjust',
    'return',
    'damage',
    'conversion',
    'purchase_return',
    'warranty_replacement',
    'recovery',
    'own_use',
    'staff'
);


--
-- Name: user_role; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.user_role AS ENUM (
    'admin',
    'cashier',
    'manager'
);


--
-- Name: create_stock_for_product(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.create_stock_for_product() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  INSERT INTO stock (product_id, quantity) VALUES (NEW.id, 0);
  RETURN NEW;
END;
$$;


--
-- Name: touch_settings_updated_at(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.touch_settings_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: audit_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.audit_log (
    id bigint NOT NULL,
    user_id bigint,
    user_name character varying(120) DEFAULT ''::character varying NOT NULL,
    action character varying(40) NOT NULL,
    entity character varying(40) NOT NULL,
    entity_id character varying(40),
    detail text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: audit_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.audit_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: audit_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.audit_log_id_seq OWNED BY public.audit_log.id;


--
-- Name: cash_movements; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cash_movements (
    id bigint NOT NULL,
    session_id bigint NOT NULL,
    user_id bigint NOT NULL,
    type public.cash_movement_type NOT NULL,
    amount numeric(14,2) NOT NULL,
    reason text,
    breakdown jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: cash_movements_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.cash_movements_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: cash_movements_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.cash_movements_id_seq OWNED BY public.cash_movements.id;


--
-- Name: cash_register; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cash_register (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    opening_cash numeric(12,2) DEFAULT 0.00 NOT NULL,
    closing_cash numeric(12,2),
    expected_cash numeric(12,2),
    difference numeric(12,2),
    opened_at timestamp with time zone DEFAULT now() NOT NULL,
    closed_at timestamp with time zone,
    opening_breakdown jsonb,
    closing_breakdown jsonb
);


--
-- Name: cash_register_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.cash_register_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: cash_register_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.cash_register_id_seq OWNED BY public.cash_register.id;


--
-- Name: categories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.categories (
    id bigint NOT NULL,
    name character varying(80) NOT NULL,
    parent_id bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: categories_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.categories_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: categories_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.categories_id_seq OWNED BY public.categories.id;


--
-- Name: conversion_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.conversion_runs (
    id bigint NOT NULL,
    conversion_id bigint,
    from_product_id bigint NOT NULL,
    to_product_id bigint NOT NULL,
    from_qty numeric(12,3) NOT NULL,
    to_qty numeric(12,3) NOT NULL,
    created_by bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: conversion_runs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.conversion_runs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: conversion_runs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.conversion_runs_id_seq OWNED BY public.conversion_runs.id;


--
-- Name: customer_payments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.customer_payments (
    id bigint NOT NULL,
    customer_id bigint NOT NULL,
    amount numeric(14,2) NOT NULL,
    method character varying(20) DEFAULT 'cash'::character varying NOT NULL,
    reference character varying(100),
    note text,
    created_by bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    receipt_no text,
    balance_before numeric(14,2),
    balance_after numeric(14,2)
);


--
-- Name: customer_payments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.customer_payments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: customer_payments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.customer_payments_id_seq OWNED BY public.customer_payments.id;


--
-- Name: customers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.customers (
    id bigint NOT NULL,
    name character varying(100) NOT NULL,
    phone character varying(15),
    address text,
    credit_limit numeric(12,2) DEFAULT 0.00 NOT NULL,
    outstanding_balance numeric(14,2) DEFAULT 0.00 NOT NULL,
    loyalty_points integer DEFAULT 0 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    opening_balance numeric(12,2) DEFAULT 0 NOT NULL,
    opening_unlinked numeric(12,2) DEFAULT 0 NOT NULL
);


--
-- Name: customers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.customers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: customers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.customers_id_seq OWNED BY public.customers.id;


--
-- Name: debt_receipt_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.debt_receipt_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: denominations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.denominations (
    id bigint NOT NULL,
    value numeric(12,2) NOT NULL,
    is_note boolean DEFAULT true NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: denominations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.denominations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: denominations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.denominations_id_seq OWNED BY public.denominations.id;


--
-- Name: expenses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.expenses (
    id bigint NOT NULL,
    category character varying(80) NOT NULL,
    amount numeric(12,2) NOT NULL,
    description text,
    paid_by bigint,
    expense_date date DEFAULT CURRENT_DATE NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    service_product_id bigint
);


--
-- Name: expenses_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.expenses_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: expenses_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.expenses_id_seq OWNED BY public.expenses.id;


--
-- Name: held_sales; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.held_sales (
    id bigint NOT NULL,
    cashier_id bigint NOT NULL,
    label character varying(120),
    sale_type character varying(20) DEFAULT 'retail'::character varying NOT NULL,
    customer_id bigint,
    discount numeric(14,2) DEFAULT 0.00 NOT NULL,
    cart jsonb NOT NULL,
    item_count integer DEFAULT 0 NOT NULL,
    total numeric(14,2) DEFAULT 0.00 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: held_sales_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.held_sales_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: held_sales_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.held_sales_id_seq OWNED BY public.held_sales.id;


--
-- Name: locker_ledger; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.locker_ledger (
    id bigint NOT NULL,
    locker_id bigint NOT NULL,
    balance_delta numeric(14,2) NOT NULL,
    kind character varying(16) NOT NULL,
    counterparty character varying(10),
    counter_till_session bigint,
    counter_locker_id bigint,
    ref_kind character varying(20),
    ref_id bigint,
    note text DEFAULT ''::text NOT NULL,
    created_by bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT locker_ledger_counterparty_check CHECK (((counterparty)::text = ANY ((ARRAY['till'::character varying, 'locker'::character varying, 'external'::character varying])::text[]))),
    CONSTRAINT locker_ledger_kind_check CHECK (((kind)::text = ANY ((ARRAY['open_balance'::character varying, 'transfer'::character varying, 'payment'::character varying, 'intake'::character varying, 'bank_charge'::character varying, 'interest'::character varying, 'adjust'::character varying])::text[])))
);


--
-- Name: locker_ledger_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.locker_ledger_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: locker_ledger_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.locker_ledger_id_seq OWNED BY public.locker_ledger.id;


--
-- Name: lockers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.lockers (
    id bigint NOT NULL,
    name text NOT NULL,
    kind character varying(10) DEFAULT 'other'::character varying NOT NULL,
    allow_negative boolean DEFAULT false NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    cashier_access boolean DEFAULT true NOT NULL,
    CONSTRAINT lockers_kind_check CHECK (((kind)::text = ANY ((ARRAY['safe'::character varying, 'bank'::character varying, 'pocket'::character varying, 'other'::character varying, 'card'::character varying])::text[])))
);


--
-- Name: lockers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.lockers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: lockers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.lockers_id_seq OWNED BY public.lockers.id;


--
-- Name: loss_recoveries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.loss_recoveries (
    id bigint NOT NULL,
    source_type character varying(16) NOT NULL,
    source_id bigint NOT NULL,
    product_id bigint NOT NULL,
    supplier_id bigint,
    outcome character varying(16) NOT NULL,
    quantity numeric(12,3) DEFAULT 1 NOT NULL,
    loss_value numeric(12,2) DEFAULT 0 NOT NULL,
    recovered_value numeric(12,2) DEFAULT 0 NOT NULL,
    note text,
    handled_by bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: loss_recoveries_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.loss_recoveries_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: loss_recoveries_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.loss_recoveries_id_seq OWNED BY public.loss_recoveries.id;


--
-- Name: money_receipt_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.money_receipt_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: money_receipts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.money_receipts (
    id bigint NOT NULL,
    receipt_no text NOT NULL,
    kind character varying(24) NOT NULL,
    from_label text DEFAULT ''::text NOT NULL,
    to_label text DEFAULT ''::text NOT NULL,
    party text DEFAULT ''::text NOT NULL,
    amount numeric(14,2) NOT NULL,
    note text DEFAULT ''::text NOT NULL,
    ref_kind character varying(24),
    ref_id bigint,
    created_by bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: money_receipts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.money_receipts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: money_receipts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.money_receipts_id_seq OWNED BY public.money_receipts.id;


--
-- Name: payments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payments (
    id bigint NOT NULL,
    sale_id bigint NOT NULL,
    method public.payment_method NOT NULL,
    amount numeric(14,2) NOT NULL,
    reference character varying(100),
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: payments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.payments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: payments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.payments_id_seq OWNED BY public.payments.id;


--
-- Name: product_conversions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.product_conversions (
    id bigint NOT NULL,
    from_product_id bigint NOT NULL,
    to_product_id bigint NOT NULL,
    ratio numeric(12,3) NOT NULL,
    note character varying(150),
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT product_conversions_check CHECK ((from_product_id <> to_product_id)),
    CONSTRAINT product_conversions_ratio_check CHECK ((ratio > (0)::numeric))
);


--
-- Name: product_conversions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.product_conversions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: product_conversions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.product_conversions_id_seq OWNED BY public.product_conversions.id;


--
-- Name: product_group_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.product_group_items (
    group_id bigint NOT NULL,
    product_id bigint NOT NULL,
    emoji character varying(16),
    sort_order integer DEFAULT 0 NOT NULL
);


--
-- Name: product_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.product_groups (
    id bigint NOT NULL,
    name character varying(80) NOT NULL,
    emoji character varying(16),
    parent_id bigint,
    sort_order integer DEFAULT 0 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: product_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.product_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: product_groups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.product_groups_id_seq OWNED BY public.product_groups.id;


--
-- Name: product_recipe_costs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.product_recipe_costs (
    id bigint NOT NULL,
    product_id bigint NOT NULL,
    label text NOT NULL,
    cost_per_unit numeric(14,4) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT product_recipe_costs_label_set CHECK ((btrim(label) <> ''::text)),
    CONSTRAINT product_recipe_costs_positive CHECK ((cost_per_unit >= (0)::numeric))
);


--
-- Name: product_recipe_costs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.product_recipe_costs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: product_recipe_costs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.product_recipe_costs_id_seq OWNED BY public.product_recipe_costs.id;


--
-- Name: product_recipes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.product_recipes (
    id bigint NOT NULL,
    product_id bigint NOT NULL,
    component_product_id bigint NOT NULL,
    qty_per_unit numeric(14,6),
    yield_units numeric(14,4),
    whole_units boolean DEFAULT false NOT NULL,
    note text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT product_recipes_no_self CHECK ((product_id <> component_product_id)),
    CONSTRAINT product_recipes_qty_xor_yield CHECK ((((qty_per_unit IS NOT NULL) AND (qty_per_unit > (0)::numeric) AND (yield_units IS NULL)) OR ((yield_units IS NOT NULL) AND (yield_units > (0)::numeric) AND (qty_per_unit IS NULL))))
);


--
-- Name: product_recipes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.product_recipes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: product_recipes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.product_recipes_id_seq OWNED BY public.product_recipes.id;


--
-- Name: products; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.products (
    id bigint NOT NULL,
    name character varying(150) NOT NULL,
    name_local character varying(150),
    barcode character varying(50),
    category_id bigint NOT NULL,
    unit_id bigint NOT NULL,
    cost_price numeric(12,2) DEFAULT 0.00 NOT NULL,
    selling_price numeric(12,2) DEFAULT 0.00 NOT NULL,
    wholesale_price numeric(12,2) DEFAULT 0.00 NOT NULL,
    tax_rate numeric(5,2) DEFAULT 0.00 NOT NULL,
    reorder_level integer DEFAULT 0 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    has_expiry boolean DEFAULT false NOT NULL,
    track_serial boolean DEFAULT false NOT NULL,
    warranty_months integer DEFAULT 0 NOT NULL,
    needs_review boolean DEFAULT false NOT NULL,
    created_by bigint,
    preferred_supplier_id bigint,
    is_service boolean DEFAULT false NOT NULL,
    pass_through boolean DEFAULT false NOT NULL
);


--
-- Name: products_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.products_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: products_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.products_id_seq OWNED BY public.products.id;


--
-- Name: purchase_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.purchase_items (
    id bigint NOT NULL,
    purchase_id bigint NOT NULL,
    product_id bigint NOT NULL,
    quantity numeric(12,3) NOT NULL,
    cost_price numeric(12,2) NOT NULL,
    selling_price numeric(12,2) NOT NULL,
    expiry_date date,
    subtotal numeric(14,2) NOT NULL,
    ordered_qty numeric(12,3),
    free_qty numeric DEFAULT 0 NOT NULL,
    discount numeric DEFAULT 0 NOT NULL,
    discount_type text DEFAULT 'fixed'::text NOT NULL,
    discount_value numeric DEFAULT 0 NOT NULL
);


--
-- Name: purchase_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.purchase_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: purchase_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.purchase_items_id_seq OWNED BY public.purchase_items.id;


--
-- Name: purchase_return_allocations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.purchase_return_allocations (
    id bigint NOT NULL,
    purchase_return_id bigint NOT NULL,
    purchase_id bigint NOT NULL,
    amount numeric(12,2) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: purchase_return_allocations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.purchase_return_allocations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: purchase_return_allocations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.purchase_return_allocations_id_seq OWNED BY public.purchase_return_allocations.id;


--
-- Name: purchase_return_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.purchase_return_items (
    id bigint NOT NULL,
    purchase_return_id bigint NOT NULL,
    product_id bigint NOT NULL,
    quantity numeric(12,3) NOT NULL,
    cost_price numeric(12,2) NOT NULL,
    subtotal numeric(14,2) NOT NULL,
    batch_id bigint
);


--
-- Name: purchase_return_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.purchase_return_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: purchase_return_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.purchase_return_items_id_seq OWNED BY public.purchase_return_items.id;


--
-- Name: purchase_returns; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.purchase_returns (
    id bigint NOT NULL,
    supplier_id bigint NOT NULL,
    reference character varying(50),
    total numeric(14,2) DEFAULT 0.00 NOT NULL,
    reason text,
    created_by bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: purchase_returns_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.purchase_returns_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: purchase_returns_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.purchase_returns_id_seq OWNED BY public.purchase_returns.id;


--
-- Name: purchases; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.purchases (
    id bigint NOT NULL,
    supplier_id bigint NOT NULL,
    invoice_no character varying(50),
    status public.purchase_status DEFAULT 'draft'::public.purchase_status NOT NULL,
    subtotal numeric(14,2) DEFAULT 0.00 NOT NULL,
    discount numeric(14,2) DEFAULT 0.00 NOT NULL,
    total numeric(14,2) DEFAULT 0.00 NOT NULL,
    paid_amount numeric(14,2) DEFAULT 0.00 NOT NULL,
    due_date date,
    received_by bigint,
    notes text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    expected_date date,
    credited_amount numeric(12,2) DEFAULT 0.00 NOT NULL,
    discount_type text DEFAULT 'fixed'::text NOT NULL,
    discount_value numeric DEFAULT 0 NOT NULL
);


--
-- Name: purchases_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.purchases_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: purchases_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.purchases_id_seq OWNED BY public.purchases.id;


--
-- Name: refresh_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.refresh_tokens (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    token_hash character varying(255) NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: refresh_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.refresh_tokens_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: refresh_tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.refresh_tokens_id_seq OWNED BY public.refresh_tokens.id;


--
-- Name: sale_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sale_items (
    id bigint NOT NULL,
    sale_id bigint NOT NULL,
    product_id bigint NOT NULL,
    quantity numeric(12,3) NOT NULL,
    unit_price numeric(12,2) NOT NULL,
    discount numeric(12,2) DEFAULT 0.00 NOT NULL,
    subtotal numeric(14,2) NOT NULL,
    cost_price numeric(12,2) DEFAULT 0.00 NOT NULL,
    returned_qty numeric(12,3) DEFAULT 0.000 NOT NULL,
    discount_type character varying(8) DEFAULT 'fixed'::character varying NOT NULL,
    discount_value numeric(12,2) DEFAULT 0 NOT NULL,
    description text,
    batch_id bigint
);


--
-- Name: sale_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sale_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sale_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sale_items_id_seq OWNED BY public.sale_items.id;


--
-- Name: sale_return_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sale_return_items (
    id bigint NOT NULL,
    return_id bigint NOT NULL,
    sale_item_id bigint NOT NULL,
    product_id bigint NOT NULL,
    quantity numeric(12,3) NOT NULL,
    refund_amount numeric(14,2) NOT NULL
);


--
-- Name: sale_return_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sale_return_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sale_return_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sale_return_items_id_seq OWNED BY public.sale_return_items.id;


--
-- Name: sale_returns; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sale_returns (
    id bigint NOT NULL,
    sale_id bigint NOT NULL,
    refund_amount numeric(14,2) DEFAULT 0.00 NOT NULL,
    credit_reduction numeric(14,2) DEFAULT 0.00 NOT NULL,
    reason text,
    created_by bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: sale_returns_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sale_returns_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sale_returns_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sale_returns_id_seq OWNED BY public.sale_returns.id;


--
-- Name: sales; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sales (
    id bigint NOT NULL,
    receipt_no character varying(20) NOT NULL,
    customer_id bigint,
    sale_type public.sale_type DEFAULT 'retail'::public.sale_type NOT NULL,
    subtotal numeric(14,2) DEFAULT 0.00 NOT NULL,
    discount numeric(14,2) DEFAULT 0.00 NOT NULL,
    tax numeric(14,2) DEFAULT 0.00 NOT NULL,
    total numeric(14,2) DEFAULT 0.00 NOT NULL,
    paid_amount numeric(14,2) DEFAULT 0.00 NOT NULL,
    change_given numeric(14,2) DEFAULT 0.00 NOT NULL,
    status public.sale_status DEFAULT 'completed'::public.sale_status NOT NULL,
    cashier_id bigint NOT NULL,
    notes text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    discount_type character varying(8) DEFAULT 'fixed'::character varying NOT NULL,
    discount_value numeric(14,2) DEFAULT 0 NOT NULL,
    rounding_to_account numeric DEFAULT 0 NOT NULL,
    customer_balance_after numeric
);


--
-- Name: sales_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sales_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sales_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.sales_id_seq OWNED BY public.sales.id;


--
-- Name: sales_receipt_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.sales_receipt_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.settings (
    id integer DEFAULT 1 NOT NULL,
    shop_name character varying(150) DEFAULT 'My Shop'::character varying NOT NULL,
    shop_name_si character varying(150),
    address text,
    phone character varying(15),
    currency_code character varying(10) DEFAULT 'LKR'::character varying NOT NULL,
    currency_symbol character varying(5) DEFAULT 'Rs.'::character varying NOT NULL,
    receipt_footer text DEFAULT 'Thank you for your purchase!'::text,
    logo_url text,
    tax_registered boolean DEFAULT false NOT NULL,
    tax_reg_no character varying(30),
    low_stock_alerts boolean DEFAULT true NOT NULL,
    default_sale_type public.sale_type DEFAULT 'retail'::public.sale_type NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    receipt_width character varying(3) DEFAULT '80'::character varying NOT NULL,
    receipt_printer character varying(100) DEFAULT ''::character varying NOT NULL,
    label_printer character varying(100) DEFAULT ''::character varying NOT NULL,
    label_width_mm integer DEFAULT 50 NOT NULL,
    label_height_mm integer DEFAULT 25 NOT NULL,
    label_gap_mm integer DEFAULT 2 NOT NULL,
    logo_data text DEFAULT ''::text NOT NULL,
    ask_to_print boolean DEFAULT true NOT NULL,
    force_pin_change boolean DEFAULT false NOT NULL,
    allow_cashier_pin_change boolean DEFAULT true NOT NULL,
    lock_timeout_minutes integer DEFAULT 0 NOT NULL,
    stock_take_enabled boolean DEFAULT true NOT NULL,
    install_id text DEFAULT ''::text NOT NULL,
    open_cash_drawer boolean DEFAULT false NOT NULL,
    drawer_kick_pin smallint DEFAULT 0 NOT NULL,
    allow_untracked_cash boolean DEFAULT true NOT NULL,
    default_locker_id bigint,
    allow_continue_last_count boolean DEFAULT true NOT NULL,
    skin text DEFAULT 'default'::text NOT NULL,
    density text DEFAULT 'comfortable'::text NOT NULL,
    receipt_style text DEFAULT 'classic'::text NOT NULL,
    skin_custom text DEFAULT ''::text NOT NULL,
    skin_radius text DEFAULT 'rounded'::text NOT NULL,
    label_name_lines integer DEFAULT 1 NOT NULL,
    label_bottom_lines integer DEFAULT 1 NOT NULL,
    label_name_font text DEFAULT '2'::text NOT NULL,
    label_bottom_font text DEFAULT '3'::text NOT NULL,
    CONSTRAINT settings_single_row CHECK ((id = 1))
);


--
-- Name: stock; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock (
    id bigint NOT NULL,
    product_id bigint NOT NULL,
    quantity numeric(14,6) DEFAULT 0.000 NOT NULL,
    last_updated timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: stock_batches; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock_batches (
    id bigint NOT NULL,
    product_id bigint NOT NULL,
    purchase_item_id bigint,
    batch_no character varying(50),
    expiry_date date,
    qty_received numeric(14,6) NOT NULL,
    qty_remaining numeric(14,6) NOT NULL,
    cost_price numeric(12,2) DEFAULT 0.00 NOT NULL,
    source character varying(20) DEFAULT 'purchase'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    selling_price numeric(12,2) DEFAULT 0.00 NOT NULL,
    CONSTRAINT stock_batches_qty_remaining_check CHECK ((qty_remaining >= (0)::numeric))
);


--
-- Name: stock_batches_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.stock_batches_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: stock_batches_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.stock_batches_id_seq OWNED BY public.stock_batches.id;


--
-- Name: stock_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.stock_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: stock_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.stock_id_seq OWNED BY public.stock.id;


--
-- Name: stock_movements; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock_movements (
    id bigint NOT NULL,
    product_id bigint NOT NULL,
    type public.stock_movement_type NOT NULL,
    quantity numeric(14,6) NOT NULL,
    reference_id bigint,
    reference_type character varying(20),
    user_id bigint NOT NULL,
    note text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    cost numeric(12,2) DEFAULT 0 NOT NULL
);


--
-- Name: stock_movements_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.stock_movements_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: stock_movements_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.stock_movements_id_seq OWNED BY public.stock_movements.id;


--
-- Name: supplier_payment_allocations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.supplier_payment_allocations (
    id bigint NOT NULL,
    payment_id bigint NOT NULL,
    purchase_id bigint NOT NULL,
    amount numeric(14,2) NOT NULL
);


--
-- Name: supplier_payment_allocations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.supplier_payment_allocations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: supplier_payment_allocations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.supplier_payment_allocations_id_seq OWNED BY public.supplier_payment_allocations.id;


--
-- Name: supplier_payments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.supplier_payments (
    id bigint NOT NULL,
    supplier_id bigint NOT NULL,
    amount numeric(14,2) NOT NULL,
    method public.payment_method DEFAULT 'cash'::public.payment_method NOT NULL,
    reference character varying(100),
    note text,
    paid_by bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: supplier_payments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.supplier_payments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: supplier_payments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.supplier_payments_id_seq OWNED BY public.supplier_payments.id;


--
-- Name: supplier_refunds; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.supplier_refunds (
    id bigint NOT NULL,
    supplier_id bigint NOT NULL,
    amount numeric(14,2) NOT NULL,
    method public.payment_method DEFAULT 'cash'::public.payment_method NOT NULL,
    reference character varying(100),
    note text,
    received_by bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT supplier_refunds_amount_check CHECK ((amount > (0)::numeric))
);


--
-- Name: supplier_refunds_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.supplier_refunds_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: supplier_refunds_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.supplier_refunds_id_seq OWNED BY public.supplier_refunds.id;


--
-- Name: suppliers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.suppliers (
    id bigint NOT NULL,
    name character varying(150) NOT NULL,
    contact_person character varying(100),
    phone character varying(15),
    address text,
    credit_days integer DEFAULT 30 NOT NULL,
    outstanding_balance numeric(14,2) DEFAULT 0.00 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    opening_balance numeric(12,2) DEFAULT 0 NOT NULL,
    opening_unlinked numeric(12,2) DEFAULT 0 NOT NULL
);


--
-- Name: suppliers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.suppliers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: suppliers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.suppliers_id_seq OWNED BY public.suppliers.id;


--
-- Name: units; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.units (
    id bigint NOT NULL,
    name character varying(30) NOT NULL,
    abbreviation character varying(10) NOT NULL,
    allow_decimal boolean DEFAULT false NOT NULL
);


--
-- Name: units_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.units_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: units_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.units_id_seq OWNED BY public.units.id;


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id bigint NOT NULL,
    name character varying(100) NOT NULL,
    phone character varying(15) NOT NULL,
    role public.user_role DEFAULT 'cashier'::public.user_role NOT NULL,
    pin_hash character varying(255) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    must_change_pin boolean DEFAULT false NOT NULL,
    receipt_printer text DEFAULT ''::text NOT NULL,
    is_system boolean DEFAULT false NOT NULL,
    can_handle_suppliers boolean DEFAULT false NOT NULL,
    can_manage_credit boolean DEFAULT false NOT NULL,
    can_see_cost boolean DEFAULT false NOT NULL
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: warranty_claims; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.warranty_claims (
    id bigint NOT NULL,
    unit_id bigint NOT NULL,
    claim_date date DEFAULT CURRENT_DATE NOT NULL,
    reason text,
    resolution character varying(16) NOT NULL,
    replacement_unit_id bigint,
    handled_by bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: warranty_claims_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.warranty_claims_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: warranty_claims_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.warranty_claims_id_seq OWNED BY public.warranty_claims.id;


--
-- Name: warranty_units; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.warranty_units (
    id bigint NOT NULL,
    product_id bigint NOT NULL,
    serial_no character varying(100),
    sale_id bigint,
    customer_id bigint,
    sold_at timestamp with time zone NOT NULL,
    warranty_months integer NOT NULL,
    warranty_until date NOT NULL,
    source character varying(16) DEFAULT 'sale'::character varying NOT NULL,
    status character varying(16) DEFAULT 'active'::character varying NOT NULL,
    replaced_by_unit_id bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: warranty_units_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.warranty_units_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: warranty_units_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.warranty_units_id_seq OWNED BY public.warranty_units.id;


--
-- Name: audit_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_log ALTER COLUMN id SET DEFAULT nextval('public.audit_log_id_seq'::regclass);


--
-- Name: cash_movements id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cash_movements ALTER COLUMN id SET DEFAULT nextval('public.cash_movements_id_seq'::regclass);


--
-- Name: cash_register id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cash_register ALTER COLUMN id SET DEFAULT nextval('public.cash_register_id_seq'::regclass);


--
-- Name: categories id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.categories ALTER COLUMN id SET DEFAULT nextval('public.categories_id_seq'::regclass);


--
-- Name: conversion_runs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversion_runs ALTER COLUMN id SET DEFAULT nextval('public.conversion_runs_id_seq'::regclass);


--
-- Name: customer_payments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_payments ALTER COLUMN id SET DEFAULT nextval('public.customer_payments_id_seq'::regclass);


--
-- Name: customers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customers ALTER COLUMN id SET DEFAULT nextval('public.customers_id_seq'::regclass);


--
-- Name: denominations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.denominations ALTER COLUMN id SET DEFAULT nextval('public.denominations_id_seq'::regclass);


--
-- Name: expenses id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.expenses ALTER COLUMN id SET DEFAULT nextval('public.expenses_id_seq'::regclass);


--
-- Name: held_sales id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.held_sales ALTER COLUMN id SET DEFAULT nextval('public.held_sales_id_seq'::regclass);


--
-- Name: locker_ledger id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.locker_ledger ALTER COLUMN id SET DEFAULT nextval('public.locker_ledger_id_seq'::regclass);


--
-- Name: lockers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.lockers ALTER COLUMN id SET DEFAULT nextval('public.lockers_id_seq'::regclass);


--
-- Name: loss_recoveries id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.loss_recoveries ALTER COLUMN id SET DEFAULT nextval('public.loss_recoveries_id_seq'::regclass);


--
-- Name: money_receipts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.money_receipts ALTER COLUMN id SET DEFAULT nextval('public.money_receipts_id_seq'::regclass);


--
-- Name: payments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments ALTER COLUMN id SET DEFAULT nextval('public.payments_id_seq'::regclass);


--
-- Name: product_conversions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_conversions ALTER COLUMN id SET DEFAULT nextval('public.product_conversions_id_seq'::regclass);


--
-- Name: product_groups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_groups ALTER COLUMN id SET DEFAULT nextval('public.product_groups_id_seq'::regclass);


--
-- Name: product_recipe_costs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_recipe_costs ALTER COLUMN id SET DEFAULT nextval('public.product_recipe_costs_id_seq'::regclass);


--
-- Name: product_recipes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_recipes ALTER COLUMN id SET DEFAULT nextval('public.product_recipes_id_seq'::regclass);


--
-- Name: products id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products ALTER COLUMN id SET DEFAULT nextval('public.products_id_seq'::regclass);


--
-- Name: purchase_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_items ALTER COLUMN id SET DEFAULT nextval('public.purchase_items_id_seq'::regclass);


--
-- Name: purchase_return_allocations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_return_allocations ALTER COLUMN id SET DEFAULT nextval('public.purchase_return_allocations_id_seq'::regclass);


--
-- Name: purchase_return_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_return_items ALTER COLUMN id SET DEFAULT nextval('public.purchase_return_items_id_seq'::regclass);


--
-- Name: purchase_returns id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_returns ALTER COLUMN id SET DEFAULT nextval('public.purchase_returns_id_seq'::regclass);


--
-- Name: purchases id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchases ALTER COLUMN id SET DEFAULT nextval('public.purchases_id_seq'::regclass);


--
-- Name: refresh_tokens id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens ALTER COLUMN id SET DEFAULT nextval('public.refresh_tokens_id_seq'::regclass);


--
-- Name: sale_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_items ALTER COLUMN id SET DEFAULT nextval('public.sale_items_id_seq'::regclass);


--
-- Name: sale_return_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_return_items ALTER COLUMN id SET DEFAULT nextval('public.sale_return_items_id_seq'::regclass);


--
-- Name: sale_returns id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_returns ALTER COLUMN id SET DEFAULT nextval('public.sale_returns_id_seq'::regclass);


--
-- Name: sales id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales ALTER COLUMN id SET DEFAULT nextval('public.sales_id_seq'::regclass);


--
-- Name: stock id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock ALTER COLUMN id SET DEFAULT nextval('public.stock_id_seq'::regclass);


--
-- Name: stock_batches id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_batches ALTER COLUMN id SET DEFAULT nextval('public.stock_batches_id_seq'::regclass);


--
-- Name: stock_movements id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_movements ALTER COLUMN id SET DEFAULT nextval('public.stock_movements_id_seq'::regclass);


--
-- Name: supplier_payment_allocations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.supplier_payment_allocations ALTER COLUMN id SET DEFAULT nextval('public.supplier_payment_allocations_id_seq'::regclass);


--
-- Name: supplier_payments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.supplier_payments ALTER COLUMN id SET DEFAULT nextval('public.supplier_payments_id_seq'::regclass);


--
-- Name: supplier_refunds id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.supplier_refunds ALTER COLUMN id SET DEFAULT nextval('public.supplier_refunds_id_seq'::regclass);


--
-- Name: suppliers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.suppliers ALTER COLUMN id SET DEFAULT nextval('public.suppliers_id_seq'::regclass);


--
-- Name: units id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.units ALTER COLUMN id SET DEFAULT nextval('public.units_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Name: warranty_claims id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warranty_claims ALTER COLUMN id SET DEFAULT nextval('public.warranty_claims_id_seq'::regclass);


--
-- Name: warranty_units id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warranty_units ALTER COLUMN id SET DEFAULT nextval('public.warranty_units_id_seq'::regclass);


--
-- Data for Name: audit_log; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cash_movements; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: cash_register; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: categories; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: conversion_runs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: customer_payments; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: customers; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: denominations; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.denominations VALUES (1, 1.00, false, true, '2026-09-27 02:46:30.023838+00');
INSERT INTO public.denominations VALUES (2, 2.00, false, true, '2026-09-27 02:46:30.023838+00');
INSERT INTO public.denominations VALUES (3, 5.00, false, true, '2026-09-27 02:46:30.023838+00');
INSERT INTO public.denominations VALUES (4, 10.00, false, true, '2026-09-27 02:46:30.023838+00');
INSERT INTO public.denominations VALUES (5, 20.00, true, true, '2026-09-27 02:46:30.023838+00');
INSERT INTO public.denominations VALUES (6, 50.00, true, true, '2026-09-27 02:46:30.023838+00');
INSERT INTO public.denominations VALUES (7, 100.00, true, true, '2026-09-27 02:46:30.023838+00');
INSERT INTO public.denominations VALUES (8, 500.00, true, true, '2026-09-27 02:46:30.023838+00');
INSERT INTO public.denominations VALUES (9, 1000.00, true, true, '2026-09-27 02:46:30.023838+00');
INSERT INTO public.denominations VALUES (10, 2000.00, true, true, '2026-09-27 02:46:30.023838+00');
INSERT INTO public.denominations VALUES (11, 5000.00, true, true, '2026-09-27 02:46:30.023838+00');


--
-- Data for Name: expenses; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: held_sales; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: locker_ledger; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: lockers; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: loss_recoveries; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: money_receipts; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: payments; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: product_conversions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: product_group_items; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: product_groups; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: product_recipe_costs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: product_recipes; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: products; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: purchase_items; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: purchase_return_allocations; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: purchase_return_items; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: purchase_returns; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: purchases; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: refresh_tokens; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sale_items; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sale_return_items; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sale_returns; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: sales; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: settings; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.settings VALUES (1, 'My Shop', NULL, NULL, NULL, 'LKR', 'Rs.', 'Thank you for your purchase!', NULL, false, NULL, true, 'retail', '2026-09-27 02:46:30.334366+00', '80', '', '', 50, 25, 2, '', true, false, true, 0, true, '42D02213', false, 0, true, NULL, true, 'default', 'comfortable', 'classic', '', 'rounded', 1, 1, '2', '3');


--
-- Data for Name: stock; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: stock_batches; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: stock_movements; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: supplier_payment_allocations; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: supplier_payments; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: supplier_refunds; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: suppliers; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: units; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.units VALUES (1, 'Piece', 'pcs', false);
INSERT INTO public.units VALUES (6, 'Packet', 'pkt', false);
INSERT INTO public.units VALUES (7, 'Bottle', 'btl', false);
INSERT INTO public.units VALUES (8, 'Box', 'box', false);
INSERT INTO public.units VALUES (9, 'Dozen', 'doz', false);
INSERT INTO public.units VALUES (2, 'Kilogram', 'kg', true);
INSERT INTO public.units VALUES (3, 'Gram', 'g', true);
INSERT INTO public.units VALUES (4, 'Litre', 'ltr', true);
INSERT INTO public.units VALUES (5, 'Millilitre', 'ml', true);


--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: warranty_claims; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: warranty_units; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Name: audit_log_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.audit_log_id_seq', 1, false);


--
-- Name: cash_movements_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.cash_movements_id_seq', 1, false);


--
-- Name: cash_register_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.cash_register_id_seq', 1, false);


--
-- Name: categories_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.categories_id_seq', 1, false);


--
-- Name: conversion_runs_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.conversion_runs_id_seq', 1, false);


--
-- Name: customer_payments_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.customer_payments_id_seq', 1, false);


--
-- Name: customers_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.customers_id_seq', 1, false);


--
-- Name: debt_receipt_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.debt_receipt_seq', 1, false);


--
-- Name: denominations_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.denominations_id_seq', 11, true);


--
-- Name: expenses_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.expenses_id_seq', 1, false);


--
-- Name: held_sales_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.held_sales_id_seq', 1, false);


--
-- Name: locker_ledger_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.locker_ledger_id_seq', 1, false);


--
-- Name: lockers_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.lockers_id_seq', 1, false);


--
-- Name: loss_recoveries_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.loss_recoveries_id_seq', 1, false);


--
-- Name: money_receipt_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.money_receipt_seq', 1, false);


--
-- Name: money_receipts_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.money_receipts_id_seq', 1, false);


--
-- Name: payments_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.payments_id_seq', 1, false);


--
-- Name: product_conversions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.product_conversions_id_seq', 1, false);


--
-- Name: product_groups_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.product_groups_id_seq', 1, false);


--
-- Name: product_recipe_costs_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.product_recipe_costs_id_seq', 1, false);


--
-- Name: product_recipes_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.product_recipes_id_seq', 1, false);


--
-- Name: products_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.products_id_seq', 1, false);


--
-- Name: purchase_items_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.purchase_items_id_seq', 1, false);


--
-- Name: purchase_return_allocations_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.purchase_return_allocations_id_seq', 1, false);


--
-- Name: purchase_return_items_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.purchase_return_items_id_seq', 1, false);


--
-- Name: purchase_returns_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.purchase_returns_id_seq', 1, false);


--
-- Name: purchases_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.purchases_id_seq', 1, false);


--
-- Name: refresh_tokens_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.refresh_tokens_id_seq', 1, false);


--
-- Name: sale_items_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.sale_items_id_seq', 1, false);


--
-- Name: sale_return_items_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.sale_return_items_id_seq', 1, false);


--
-- Name: sale_returns_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.sale_returns_id_seq', 1, false);


--
-- Name: sales_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.sales_id_seq', 1, false);


--
-- Name: sales_receipt_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.sales_receipt_seq', 1, false);


--
-- Name: stock_batches_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.stock_batches_id_seq', 1, false);


--
-- Name: stock_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.stock_id_seq', 1, false);


--
-- Name: stock_movements_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.stock_movements_id_seq', 1, false);


--
-- Name: supplier_payment_allocations_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.supplier_payment_allocations_id_seq', 1, false);


--
-- Name: supplier_payments_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.supplier_payments_id_seq', 1, false);


--
-- Name: supplier_refunds_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.supplier_refunds_id_seq', 1, false);


--
-- Name: suppliers_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.suppliers_id_seq', 1, false);


--
-- Name: units_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.units_id_seq', 9, true);


--
-- Name: users_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.users_id_seq', 1, false);


--
-- Name: warranty_claims_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.warranty_claims_id_seq', 1, false);


--
-- Name: warranty_units_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.warranty_units_id_seq', 1, false);


--
-- Name: audit_log audit_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_log
    ADD CONSTRAINT audit_log_pkey PRIMARY KEY (id);


--
-- Name: cash_movements cash_movements_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cash_movements
    ADD CONSTRAINT cash_movements_pkey PRIMARY KEY (id);


--
-- Name: cash_register cash_register_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cash_register
    ADD CONSTRAINT cash_register_pkey PRIMARY KEY (id);


--
-- Name: categories categories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_pkey PRIMARY KEY (id);


--
-- Name: conversion_runs conversion_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversion_runs
    ADD CONSTRAINT conversion_runs_pkey PRIMARY KEY (id);


--
-- Name: customer_payments customer_payments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_payments
    ADD CONSTRAINT customer_payments_pkey PRIMARY KEY (id);


--
-- Name: customers customers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customers
    ADD CONSTRAINT customers_pkey PRIMARY KEY (id);


--
-- Name: denominations denominations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.denominations
    ADD CONSTRAINT denominations_pkey PRIMARY KEY (id);


--
-- Name: denominations denominations_value_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.denominations
    ADD CONSTRAINT denominations_value_key UNIQUE (value);


--
-- Name: expenses expenses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.expenses
    ADD CONSTRAINT expenses_pkey PRIMARY KEY (id);


--
-- Name: held_sales held_sales_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.held_sales
    ADD CONSTRAINT held_sales_pkey PRIMARY KEY (id);


--
-- Name: locker_ledger locker_ledger_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.locker_ledger
    ADD CONSTRAINT locker_ledger_pkey PRIMARY KEY (id);


--
-- Name: lockers lockers_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.lockers
    ADD CONSTRAINT lockers_name_key UNIQUE (name);


--
-- Name: lockers lockers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.lockers
    ADD CONSTRAINT lockers_pkey PRIMARY KEY (id);


--
-- Name: loss_recoveries loss_recoveries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.loss_recoveries
    ADD CONSTRAINT loss_recoveries_pkey PRIMARY KEY (id);


--
-- Name: money_receipts money_receipts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.money_receipts
    ADD CONSTRAINT money_receipts_pkey PRIMARY KEY (id);


--
-- Name: money_receipts money_receipts_receipt_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.money_receipts
    ADD CONSTRAINT money_receipts_receipt_no_key UNIQUE (receipt_no);


--
-- Name: payments payments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_pkey PRIMARY KEY (id);


--
-- Name: product_conversions product_conversions_from_product_id_to_product_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_conversions
    ADD CONSTRAINT product_conversions_from_product_id_to_product_id_key UNIQUE (from_product_id, to_product_id);


--
-- Name: product_conversions product_conversions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_conversions
    ADD CONSTRAINT product_conversions_pkey PRIMARY KEY (id);


--
-- Name: product_group_items product_group_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_group_items
    ADD CONSTRAINT product_group_items_pkey PRIMARY KEY (group_id, product_id);


--
-- Name: product_groups product_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_groups
    ADD CONSTRAINT product_groups_pkey PRIMARY KEY (id);


--
-- Name: product_recipe_costs product_recipe_costs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_recipe_costs
    ADD CONSTRAINT product_recipe_costs_pkey PRIMARY KEY (id);


--
-- Name: product_recipe_costs product_recipe_costs_unique_label; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_recipe_costs
    ADD CONSTRAINT product_recipe_costs_unique_label UNIQUE (product_id, label);


--
-- Name: product_recipes product_recipes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_recipes
    ADD CONSTRAINT product_recipes_pkey PRIMARY KEY (id);


--
-- Name: product_recipes product_recipes_unique_component; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_recipes
    ADD CONSTRAINT product_recipes_unique_component UNIQUE (product_id, component_product_id);


--
-- Name: products products_barcode_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_barcode_key UNIQUE (barcode);


--
-- Name: products products_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_pkey PRIMARY KEY (id);


--
-- Name: purchase_items purchase_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_items
    ADD CONSTRAINT purchase_items_pkey PRIMARY KEY (id);


--
-- Name: purchase_return_allocations purchase_return_allocations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_return_allocations
    ADD CONSTRAINT purchase_return_allocations_pkey PRIMARY KEY (id);


--
-- Name: purchase_return_items purchase_return_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_return_items
    ADD CONSTRAINT purchase_return_items_pkey PRIMARY KEY (id);


--
-- Name: purchase_returns purchase_returns_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_returns
    ADD CONSTRAINT purchase_returns_pkey PRIMARY KEY (id);


--
-- Name: purchases purchases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchases
    ADD CONSTRAINT purchases_pkey PRIMARY KEY (id);


--
-- Name: refresh_tokens refresh_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_pkey PRIMARY KEY (id);


--
-- Name: refresh_tokens refresh_tokens_token_hash_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_token_hash_key UNIQUE (token_hash);


--
-- Name: sale_items sale_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_items
    ADD CONSTRAINT sale_items_pkey PRIMARY KEY (id);


--
-- Name: sale_return_items sale_return_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_return_items
    ADD CONSTRAINT sale_return_items_pkey PRIMARY KEY (id);


--
-- Name: sale_returns sale_returns_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_returns
    ADD CONSTRAINT sale_returns_pkey PRIMARY KEY (id);


--
-- Name: sales sales_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales
    ADD CONSTRAINT sales_pkey PRIMARY KEY (id);


--
-- Name: sales sales_receipt_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales
    ADD CONSTRAINT sales_receipt_no_key UNIQUE (receipt_no);


--
-- Name: settings settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settings
    ADD CONSTRAINT settings_pkey PRIMARY KEY (id);


--
-- Name: stock_batches stock_batches_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_batches
    ADD CONSTRAINT stock_batches_pkey PRIMARY KEY (id);


--
-- Name: stock_movements stock_movements_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_movements
    ADD CONSTRAINT stock_movements_pkey PRIMARY KEY (id);


--
-- Name: stock stock_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock
    ADD CONSTRAINT stock_pkey PRIMARY KEY (id);


--
-- Name: stock stock_product_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock
    ADD CONSTRAINT stock_product_id_key UNIQUE (product_id);


--
-- Name: supplier_payment_allocations supplier_payment_allocations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.supplier_payment_allocations
    ADD CONSTRAINT supplier_payment_allocations_pkey PRIMARY KEY (id);


--
-- Name: supplier_payments supplier_payments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.supplier_payments
    ADD CONSTRAINT supplier_payments_pkey PRIMARY KEY (id);


--
-- Name: supplier_refunds supplier_refunds_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.supplier_refunds
    ADD CONSTRAINT supplier_refunds_pkey PRIMARY KEY (id);


--
-- Name: suppliers suppliers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.suppliers
    ADD CONSTRAINT suppliers_pkey PRIMARY KEY (id);


--
-- Name: units units_abbreviation_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.units
    ADD CONSTRAINT units_abbreviation_key UNIQUE (abbreviation);


--
-- Name: units units_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.units
    ADD CONSTRAINT units_name_key UNIQUE (name);


--
-- Name: units units_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.units
    ADD CONSTRAINT units_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: warranty_claims warranty_claims_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warranty_claims
    ADD CONSTRAINT warranty_claims_pkey PRIMARY KEY (id);


--
-- Name: warranty_units warranty_units_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warranty_units
    ADD CONSTRAINT warranty_units_pkey PRIMARY KEY (id);


--
-- Name: warranty_units warranty_units_serial_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warranty_units
    ADD CONSTRAINT warranty_units_serial_no_key UNIQUE (serial_no);


--
-- Name: customer_payments_receipt_no_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX customer_payments_receipt_no_key ON public.customer_payments USING btree (receipt_no);


--
-- Name: idx_audit_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_created ON public.audit_log USING btree (created_at DESC);


--
-- Name: idx_audit_entity; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_entity ON public.audit_log USING btree (entity);


--
-- Name: idx_audit_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_user ON public.audit_log USING btree (user_id);


--
-- Name: idx_cash_movements_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cash_movements_created ON public.cash_movements USING btree (created_at DESC);


--
-- Name: idx_cash_movements_session; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cash_movements_session ON public.cash_movements USING btree (session_id);


--
-- Name: idx_cash_register_open_session; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_cash_register_open_session ON public.cash_register USING btree (user_id) WHERE (closed_at IS NULL);


--
-- Name: idx_cash_register_opened_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cash_register_opened_at ON public.cash_register USING btree (opened_at DESC);


--
-- Name: idx_cash_register_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cash_register_user_id ON public.cash_register USING btree (user_id);


--
-- Name: idx_categories_parent_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_categories_parent_id ON public.categories USING btree (parent_id);


--
-- Name: idx_conversion_runs_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_conversion_runs_created_at ON public.conversion_runs USING btree (created_at DESC);


--
-- Name: idx_customer_payments_customer; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_customer_payments_customer ON public.customer_payments USING btree (customer_id, created_at);


--
-- Name: idx_customers_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_customers_name ON public.customers USING btree (name);


--
-- Name: idx_customers_phone; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_customers_phone ON public.customers USING btree (phone);


--
-- Name: idx_expenses_category; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_expenses_category ON public.expenses USING btree (category);


--
-- Name: idx_expenses_expense_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_expenses_expense_date ON public.expenses USING btree (expense_date DESC);


--
-- Name: idx_expenses_paid_by; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_expenses_paid_by ON public.expenses USING btree (paid_by);


--
-- Name: idx_expenses_service_product; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_expenses_service_product ON public.expenses USING btree (service_product_id) WHERE (service_product_id IS NOT NULL);


--
-- Name: idx_held_sales_cashier; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_held_sales_cashier ON public.held_sales USING btree (cashier_id, created_at DESC);


--
-- Name: idx_locker_ledger_locker; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_locker_ledger_locker ON public.locker_ledger USING btree (locker_id);


--
-- Name: idx_loss_recoveries_source; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_loss_recoveries_source ON public.loss_recoveries USING btree (source_type, source_id);


--
-- Name: idx_money_receipts_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_money_receipts_created ON public.money_receipts USING btree (created_at DESC);


--
-- Name: idx_money_receipts_kind; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_money_receipts_kind ON public.money_receipts USING btree (kind);


--
-- Name: idx_payments_method; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_payments_method ON public.payments USING btree (method);


--
-- Name: idx_payments_sale_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_payments_sale_id ON public.payments USING btree (sale_id);


--
-- Name: idx_pgi_product; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_pgi_product ON public.product_group_items USING btree (product_id);


--
-- Name: idx_pr_alloc_purchase; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_pr_alloc_purchase ON public.purchase_return_allocations USING btree (purchase_id);


--
-- Name: idx_pr_alloc_return; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_pr_alloc_return ON public.purchase_return_allocations USING btree (purchase_return_id);


--
-- Name: idx_product_groups_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_product_groups_parent ON public.product_groups USING btree (parent_id);


--
-- Name: idx_product_recipe_costs_product; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_product_recipe_costs_product ON public.product_recipe_costs USING btree (product_id);


--
-- Name: idx_product_recipes_product; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_product_recipes_product ON public.product_recipes USING btree (product_id);


--
-- Name: idx_products_barcode; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_products_barcode ON public.products USING btree (barcode);


--
-- Name: idx_products_category_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_products_category_id ON public.products USING btree (category_id);


--
-- Name: idx_products_is_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_products_is_active ON public.products USING btree (is_active);


--
-- Name: idx_products_name_search; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_products_name_search ON public.products USING gin (to_tsvector('simple'::regconfig, (name)::text));


--
-- Name: idx_products_name_squashed_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_products_name_squashed_trgm ON public.products USING gin (translate(lower((name)::text), ' -_/.'::text, ''::text) public.gin_trgm_ops);


--
-- Name: idx_products_name_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_products_name_trgm ON public.products USING gin (lower((name)::text) public.gin_trgm_ops);


--
-- Name: idx_products_preferred_supplier; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_products_preferred_supplier ON public.products USING btree (preferred_supplier_id);


--
-- Name: idx_purchase_items_expiry_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_purchase_items_expiry_date ON public.purchase_items USING btree (expiry_date);


--
-- Name: idx_purchase_items_product_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_purchase_items_product_id ON public.purchase_items USING btree (product_id);


--
-- Name: idx_purchase_items_purchase_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_purchase_items_purchase_id ON public.purchase_items USING btree (purchase_id);


--
-- Name: idx_purchase_return_items_pr_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_purchase_return_items_pr_id ON public.purchase_return_items USING btree (purchase_return_id);


--
-- Name: idx_purchase_returns_supplier_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_purchase_returns_supplier_id ON public.purchase_returns USING btree (supplier_id);


--
-- Name: idx_purchases_due_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_purchases_due_date ON public.purchases USING btree (due_date);


--
-- Name: idx_purchases_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_purchases_status ON public.purchases USING btree (status);


--
-- Name: idx_purchases_supplier_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_purchases_supplier_id ON public.purchases USING btree (supplier_id);


--
-- Name: idx_refresh_tokens_expires_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_refresh_tokens_expires_at ON public.refresh_tokens USING btree (expires_at);


--
-- Name: idx_refresh_tokens_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_refresh_tokens_user_id ON public.refresh_tokens USING btree (user_id);


--
-- Name: idx_sale_items_batch; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sale_items_batch ON public.sale_items USING btree (batch_id) WHERE (batch_id IS NOT NULL);


--
-- Name: idx_sale_items_product_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sale_items_product_id ON public.sale_items USING btree (product_id);


--
-- Name: idx_sale_items_sale_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sale_items_sale_id ON public.sale_items USING btree (sale_id);


--
-- Name: idx_sale_return_items_return_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sale_return_items_return_id ON public.sale_return_items USING btree (return_id);


--
-- Name: idx_sale_returns_sale_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sale_returns_sale_id ON public.sale_returns USING btree (sale_id);


--
-- Name: idx_sales_cashier_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sales_cashier_id ON public.sales USING btree (cashier_id);


--
-- Name: idx_sales_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sales_created_at ON public.sales USING btree (created_at DESC);


--
-- Name: idx_sales_customer_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sales_customer_id ON public.sales USING btree (customer_id);


--
-- Name: idx_sales_sale_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sales_sale_type ON public.sales USING btree (sale_type);


--
-- Name: idx_sales_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sales_status ON public.sales USING btree (status);


--
-- Name: idx_stock_batches_expiry; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_stock_batches_expiry ON public.stock_batches USING btree (expiry_date) WHERE (qty_remaining > (0)::numeric);


--
-- Name: idx_stock_batches_fefo; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_stock_batches_fefo ON public.stock_batches USING btree (product_id, expiry_date, id) WHERE (qty_remaining > (0)::numeric);


--
-- Name: idx_stock_movements_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_stock_movements_created_at ON public.stock_movements USING btree (created_at DESC);


--
-- Name: idx_stock_movements_product_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_stock_movements_product_id ON public.stock_movements USING btree (product_id);


--
-- Name: idx_stock_movements_ref; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_stock_movements_ref ON public.stock_movements USING btree (reference_type, reference_id);


--
-- Name: idx_stock_movements_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_stock_movements_type ON public.stock_movements USING btree (type);


--
-- Name: idx_supplier_payment_alloc_payment; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_supplier_payment_alloc_payment ON public.supplier_payment_allocations USING btree (payment_id);


--
-- Name: idx_supplier_payment_alloc_purchase; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_supplier_payment_alloc_purchase ON public.supplier_payment_allocations USING btree (purchase_id);


--
-- Name: idx_supplier_payments_supplier; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_supplier_payments_supplier ON public.supplier_payments USING btree (supplier_id);


--
-- Name: idx_supplier_refunds_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_supplier_refunds_created ON public.supplier_refunds USING btree (created_at);


--
-- Name: idx_supplier_refunds_supplier; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_supplier_refunds_supplier ON public.supplier_refunds USING btree (supplier_id);


--
-- Name: idx_suppliers_is_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_suppliers_is_active ON public.suppliers USING btree (is_active);


--
-- Name: idx_users_is_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_is_active ON public.users USING btree (is_active);


--
-- Name: idx_users_phone; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_users_phone ON public.users USING btree (phone) WHERE (is_active = true);


--
-- Name: idx_warranty_claims_unit; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_warranty_claims_unit ON public.warranty_claims USING btree (unit_id);


--
-- Name: idx_warranty_units_product; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_warranty_units_product ON public.warranty_units USING btree (product_id);


--
-- Name: idx_warranty_units_sale; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_warranty_units_sale ON public.warranty_units USING btree (sale_id);


--
-- Name: products trg_product_stock; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_product_stock AFTER INSERT ON public.products FOR EACH ROW EXECUTE FUNCTION public.create_stock_for_product();


--
-- Name: settings trg_settings_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_settings_updated_at BEFORE UPDATE ON public.settings FOR EACH ROW EXECUTE FUNCTION public.touch_settings_updated_at();


--
-- Name: audit_log audit_log_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_log
    ADD CONSTRAINT audit_log_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: cash_movements cash_movements_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cash_movements
    ADD CONSTRAINT cash_movements_session_id_fkey FOREIGN KEY (session_id) REFERENCES public.cash_register(id) ON DELETE CASCADE;


--
-- Name: cash_movements cash_movements_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cash_movements
    ADD CONSTRAINT cash_movements_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: cash_register cash_register_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cash_register
    ADD CONSTRAINT cash_register_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: categories categories_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.categories(id) ON DELETE SET NULL;


--
-- Name: conversion_runs conversion_runs_conversion_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversion_runs
    ADD CONSTRAINT conversion_runs_conversion_id_fkey FOREIGN KEY (conversion_id) REFERENCES public.product_conversions(id) ON DELETE SET NULL;


--
-- Name: conversion_runs conversion_runs_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversion_runs
    ADD CONSTRAINT conversion_runs_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: conversion_runs conversion_runs_from_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversion_runs
    ADD CONSTRAINT conversion_runs_from_product_id_fkey FOREIGN KEY (from_product_id) REFERENCES public.products(id) ON DELETE RESTRICT;


--
-- Name: conversion_runs conversion_runs_to_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversion_runs
    ADD CONSTRAINT conversion_runs_to_product_id_fkey FOREIGN KEY (to_product_id) REFERENCES public.products(id) ON DELETE RESTRICT;


--
-- Name: customer_payments customer_payments_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_payments
    ADD CONSTRAINT customer_payments_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: customer_payments customer_payments_customer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_payments
    ADD CONSTRAINT customer_payments_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customers(id) ON DELETE CASCADE;


--
-- Name: expenses expenses_paid_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.expenses
    ADD CONSTRAINT expenses_paid_by_fkey FOREIGN KEY (paid_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: expenses expenses_service_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.expenses
    ADD CONSTRAINT expenses_service_product_id_fkey FOREIGN KEY (service_product_id) REFERENCES public.products(id) ON DELETE SET NULL;


--
-- Name: held_sales held_sales_cashier_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.held_sales
    ADD CONSTRAINT held_sales_cashier_id_fkey FOREIGN KEY (cashier_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: held_sales held_sales_customer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.held_sales
    ADD CONSTRAINT held_sales_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customers(id) ON DELETE SET NULL;


--
-- Name: locker_ledger locker_ledger_counter_locker_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.locker_ledger
    ADD CONSTRAINT locker_ledger_counter_locker_id_fkey FOREIGN KEY (counter_locker_id) REFERENCES public.lockers(id);


--
-- Name: locker_ledger locker_ledger_locker_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.locker_ledger
    ADD CONSTRAINT locker_ledger_locker_id_fkey FOREIGN KEY (locker_id) REFERENCES public.lockers(id);


--
-- Name: loss_recoveries loss_recoveries_handled_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.loss_recoveries
    ADD CONSTRAINT loss_recoveries_handled_by_fkey FOREIGN KEY (handled_by) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: loss_recoveries loss_recoveries_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.loss_recoveries
    ADD CONSTRAINT loss_recoveries_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE RESTRICT;


--
-- Name: loss_recoveries loss_recoveries_supplier_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.loss_recoveries
    ADD CONSTRAINT loss_recoveries_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES public.suppliers(id) ON DELETE SET NULL;


--
-- Name: payments payments_sale_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_sale_id_fkey FOREIGN KEY (sale_id) REFERENCES public.sales(id) ON DELETE CASCADE;


--
-- Name: product_conversions product_conversions_from_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_conversions
    ADD CONSTRAINT product_conversions_from_product_id_fkey FOREIGN KEY (from_product_id) REFERENCES public.products(id) ON DELETE CASCADE;


--
-- Name: product_conversions product_conversions_to_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_conversions
    ADD CONSTRAINT product_conversions_to_product_id_fkey FOREIGN KEY (to_product_id) REFERENCES public.products(id) ON DELETE CASCADE;


--
-- Name: product_group_items product_group_items_group_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_group_items
    ADD CONSTRAINT product_group_items_group_id_fkey FOREIGN KEY (group_id) REFERENCES public.product_groups(id) ON DELETE CASCADE;


--
-- Name: product_group_items product_group_items_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_group_items
    ADD CONSTRAINT product_group_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE CASCADE;


--
-- Name: product_groups product_groups_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_groups
    ADD CONSTRAINT product_groups_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.product_groups(id) ON DELETE CASCADE;


--
-- Name: product_recipe_costs product_recipe_costs_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_recipe_costs
    ADD CONSTRAINT product_recipe_costs_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE CASCADE;


--
-- Name: product_recipes product_recipes_component_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_recipes
    ADD CONSTRAINT product_recipes_component_product_id_fkey FOREIGN KEY (component_product_id) REFERENCES public.products(id) ON DELETE RESTRICT;


--
-- Name: product_recipes product_recipes_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_recipes
    ADD CONSTRAINT product_recipes_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE CASCADE;


--
-- Name: products products_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_category_id_fkey FOREIGN KEY (category_id) REFERENCES public.categories(id) ON DELETE RESTRICT;


--
-- Name: products products_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id);


--
-- Name: products products_preferred_supplier_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_preferred_supplier_id_fkey FOREIGN KEY (preferred_supplier_id) REFERENCES public.suppliers(id) ON DELETE SET NULL;


--
-- Name: products products_unit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_unit_id_fkey FOREIGN KEY (unit_id) REFERENCES public.units(id) ON DELETE RESTRICT;


--
-- Name: purchase_items purchase_items_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_items
    ADD CONSTRAINT purchase_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE RESTRICT;


--
-- Name: purchase_items purchase_items_purchase_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_items
    ADD CONSTRAINT purchase_items_purchase_id_fkey FOREIGN KEY (purchase_id) REFERENCES public.purchases(id) ON DELETE CASCADE;


--
-- Name: purchase_return_allocations purchase_return_allocations_purchase_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_return_allocations
    ADD CONSTRAINT purchase_return_allocations_purchase_id_fkey FOREIGN KEY (purchase_id) REFERENCES public.purchases(id) ON DELETE CASCADE;


--
-- Name: purchase_return_allocations purchase_return_allocations_purchase_return_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_return_allocations
    ADD CONSTRAINT purchase_return_allocations_purchase_return_id_fkey FOREIGN KEY (purchase_return_id) REFERENCES public.purchase_returns(id) ON DELETE CASCADE;


--
-- Name: purchase_return_items purchase_return_items_batch_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_return_items
    ADD CONSTRAINT purchase_return_items_batch_id_fkey FOREIGN KEY (batch_id) REFERENCES public.stock_batches(id) ON DELETE SET NULL;


--
-- Name: purchase_return_items purchase_return_items_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_return_items
    ADD CONSTRAINT purchase_return_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE RESTRICT;


--
-- Name: purchase_return_items purchase_return_items_purchase_return_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_return_items
    ADD CONSTRAINT purchase_return_items_purchase_return_id_fkey FOREIGN KEY (purchase_return_id) REFERENCES public.purchase_returns(id) ON DELETE CASCADE;


--
-- Name: purchase_returns purchase_returns_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_returns
    ADD CONSTRAINT purchase_returns_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: purchase_returns purchase_returns_supplier_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchase_returns
    ADD CONSTRAINT purchase_returns_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES public.suppliers(id) ON DELETE RESTRICT;


--
-- Name: purchases purchases_received_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchases
    ADD CONSTRAINT purchases_received_by_fkey FOREIGN KEY (received_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: purchases purchases_supplier_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.purchases
    ADD CONSTRAINT purchases_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES public.suppliers(id) ON DELETE RESTRICT;


--
-- Name: refresh_tokens refresh_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: sale_items sale_items_batch_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_items
    ADD CONSTRAINT sale_items_batch_id_fkey FOREIGN KEY (batch_id) REFERENCES public.stock_batches(id) ON DELETE SET NULL;


--
-- Name: sale_items sale_items_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_items
    ADD CONSTRAINT sale_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE RESTRICT;


--
-- Name: sale_items sale_items_sale_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_items
    ADD CONSTRAINT sale_items_sale_id_fkey FOREIGN KEY (sale_id) REFERENCES public.sales(id) ON DELETE CASCADE;


--
-- Name: sale_return_items sale_return_items_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_return_items
    ADD CONSTRAINT sale_return_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE RESTRICT;


--
-- Name: sale_return_items sale_return_items_return_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_return_items
    ADD CONSTRAINT sale_return_items_return_id_fkey FOREIGN KEY (return_id) REFERENCES public.sale_returns(id) ON DELETE CASCADE;


--
-- Name: sale_return_items sale_return_items_sale_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_return_items
    ADD CONSTRAINT sale_return_items_sale_item_id_fkey FOREIGN KEY (sale_item_id) REFERENCES public.sale_items(id) ON DELETE RESTRICT;


--
-- Name: sale_returns sale_returns_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_returns
    ADD CONSTRAINT sale_returns_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: sale_returns sale_returns_sale_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_returns
    ADD CONSTRAINT sale_returns_sale_id_fkey FOREIGN KEY (sale_id) REFERENCES public.sales(id) ON DELETE CASCADE;


--
-- Name: sales sales_cashier_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales
    ADD CONSTRAINT sales_cashier_id_fkey FOREIGN KEY (cashier_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: sales sales_customer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales
    ADD CONSTRAINT sales_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customers(id) ON DELETE SET NULL;


--
-- Name: settings settings_default_locker_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settings
    ADD CONSTRAINT settings_default_locker_id_fkey FOREIGN KEY (default_locker_id) REFERENCES public.lockers(id) ON DELETE SET NULL;


--
-- Name: stock_batches stock_batches_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_batches
    ADD CONSTRAINT stock_batches_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE CASCADE;


--
-- Name: stock_batches stock_batches_purchase_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_batches
    ADD CONSTRAINT stock_batches_purchase_item_id_fkey FOREIGN KEY (purchase_item_id) REFERENCES public.purchase_items(id) ON DELETE SET NULL;


--
-- Name: stock_movements stock_movements_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_movements
    ADD CONSTRAINT stock_movements_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE RESTRICT;


--
-- Name: stock_movements stock_movements_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_movements
    ADD CONSTRAINT stock_movements_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: stock stock_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock
    ADD CONSTRAINT stock_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE CASCADE;


--
-- Name: supplier_payment_allocations supplier_payment_allocations_payment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.supplier_payment_allocations
    ADD CONSTRAINT supplier_payment_allocations_payment_id_fkey FOREIGN KEY (payment_id) REFERENCES public.supplier_payments(id) ON DELETE CASCADE;


--
-- Name: supplier_payment_allocations supplier_payment_allocations_purchase_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.supplier_payment_allocations
    ADD CONSTRAINT supplier_payment_allocations_purchase_id_fkey FOREIGN KEY (purchase_id) REFERENCES public.purchases(id);


--
-- Name: supplier_payments supplier_payments_paid_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.supplier_payments
    ADD CONSTRAINT supplier_payments_paid_by_fkey FOREIGN KEY (paid_by) REFERENCES public.users(id);


--
-- Name: supplier_payments supplier_payments_supplier_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.supplier_payments
    ADD CONSTRAINT supplier_payments_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES public.suppliers(id);


--
-- Name: supplier_refunds supplier_refunds_received_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.supplier_refunds
    ADD CONSTRAINT supplier_refunds_received_by_fkey FOREIGN KEY (received_by) REFERENCES public.users(id);


--
-- Name: supplier_refunds supplier_refunds_supplier_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.supplier_refunds
    ADD CONSTRAINT supplier_refunds_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES public.suppliers(id);


--
-- Name: warranty_claims warranty_claims_handled_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warranty_claims
    ADD CONSTRAINT warranty_claims_handled_by_fkey FOREIGN KEY (handled_by) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: warranty_claims warranty_claims_replacement_unit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warranty_claims
    ADD CONSTRAINT warranty_claims_replacement_unit_id_fkey FOREIGN KEY (replacement_unit_id) REFERENCES public.warranty_units(id) ON DELETE SET NULL;


--
-- Name: warranty_claims warranty_claims_unit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warranty_claims
    ADD CONSTRAINT warranty_claims_unit_id_fkey FOREIGN KEY (unit_id) REFERENCES public.warranty_units(id) ON DELETE CASCADE;


--
-- Name: warranty_units warranty_units_customer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warranty_units
    ADD CONSTRAINT warranty_units_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customers(id) ON DELETE SET NULL;


--
-- Name: warranty_units warranty_units_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warranty_units
    ADD CONSTRAINT warranty_units_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE RESTRICT;


--
-- Name: warranty_units warranty_units_replaced_by_unit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warranty_units
    ADD CONSTRAINT warranty_units_replaced_by_unit_id_fkey FOREIGN KEY (replaced_by_unit_id) REFERENCES public.warranty_units(id) ON DELETE SET NULL;


--
-- Name: warranty_units warranty_units_sale_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warranty_units
    ADD CONSTRAINT warranty_units_sale_id_fkey FOREIGN KEY (sale_id) REFERENCES public.sales(id) ON DELETE SET NULL;


--
-- PostgreSQL database dump complete
--


-- install_id must be random PER DATABASE (support PIN is derived from it).
UPDATE settings SET install_id = upper(substr(md5(random()::text || clock_timestamp()::text), 1, 8));
-- +goose StatementEnd
