package sitemanager

import (
	"github.com/tinywasm/model"
)

var SiteModel = model.Definition{
	Name: "site",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "slug", Type: model.Text(), NotNull: true, DB: &model.FieldDB{Unique: true}},
		{Name: "name", Type: model.Text(), NotNull: true},
		{Name: "domain", Type: model.Text()},
		{Name: "domain_expires_at", Type: model.Int()},
		{Name: "plan", Type: model.Text()},
		{Name: "theme", Type: model.Text(), NotNull: true},
		{Name: "status", Type: model.Int()},
		{Name: "dirty", Type: model.Bool()},
		{Name: "published_at", Type: model.Int()},
		{Name: "published_ref", Type: model.Text()},
	},
}

var SiteMemberModel = model.Definition{
	Name: "site_member",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "site_id", Type: model.Text(), NotNull: true, Ref: &SiteModel, DB: &model.FieldDB{RefColumn: "id"}},
		{Name: "user_id", Type: model.Text(), NotNull: true},
		{Name: "role", Type: model.Int()},
	},
}

var PlanModel = model.Definition{
	Name: "plan",
	Fields: model.Fields{
		{Name: "name", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "max_pages", Type: model.Int()},
		{Name: "max_images", Type: model.Int()},
		{Name: "custom_domain", Type: model.Bool()},
	},
}

var AccessRequestModel = model.Definition{
	Name: "access_request",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "email", Type: model.Text(), NotNull: true},
		{Name: "name", Type: model.Text()},
		{Name: "message", Type: model.Text()},
		{Name: "created_at", Type: model.Int()},
		{Name: "status", Type: model.Int()},
	},
}
