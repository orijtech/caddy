// Copyright 2018 Light Code Labs, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package caddytls

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

const dimensionless = "1"

var (
	tagCertName, _ = tag.NewKey("caddytls/cert_name")

	mRevokeCalls     = stats.Int64("caddytls/revoke", "The number of Revoke invocations", dimensionless)
	mRevokeFailures  = stats.Int64("caddytls/revoke_failed", "The number of failed Revoke invocations", dimensionless)
	mRevokeSuccesses = stats.Int64("caddytls/revoke_successful", "The number of successful Revoke invocations", dimensionless)

	mSiteExistsErrors    = stats.Int64("caddytls/site_exists_errors", "The number of errors related to SiteExist calls", dimensionless)
	mSiteLoadErrors      = stats.Int64("caddytls/site_load_errors", "The number of errors related to LoadSite calls", dimensionless)
	mSiteNotExistsErrors = stats.Int64("caddytls/site_not_exist", "The number of times the site is not found in storage", dimensionless)

	mFailedCertFileDeletions = stats.Int64("caddytls/failed_cert_deletions", "The number of failed certificate deletions", dimensionless)

	mObtainCalls     = stats.Int64("caddytls/obtain", "The number of Obtain invocations", dimensionless)
	mObtainFailures  = stats.Int64("caddytls/obtain_failures", "The number of failed Obtain invocations", dimensionless)
	mObtainSuccesses = stats.Int64("caddytls/obtain_successes", "The number of successful Obtain invocations", dimensionless)

	mObtainBlankCertificates = stats.Int64("caddytls/obtain_blank_certs", "The number of blank Obtain certificates", dimensionless)

	mSaveCertErrors = stats.Int64("caddytls/save_cert_errors", "The number of failed save certificate errors", dimensionless)

	mRenewCalls     = stats.Int64("caddytls/renew", "The number of Renew invocations", dimensionless)
	mRenewFailures  = stats.Int64("caddytls/renew_failures", "The number of failed Renew invocations", dimensionless)
	mRenewSuccesses = stats.Int64("caddytls/renew_successes", "The number of successful Renew invocations", dimensionless)
)

var Views = []*view.View{
	{
		Name:        "caddytls/revoke_calls",
		Description: "View for the number of revoke calls",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{tagCertName},
		Measure:     mRevokeCalls,
	},
	{
		Name:        "caddytls/revoke_successful",
		Description: "View for the number of successful revoke calls",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{tagCertName},
		Measure:     mRevokeSuccesses,
	},
	{
		Name:        "caddytls/revoke_failed",
		Description: "View for the number of failed revoke calls",
		Aggregation: view.Count(),
		Measure:     mRevokeFailures,
		TagKeys:     []tag.Key{tagCertName},
	},
	{
		Name:        "caddytls/site_exists_errors",
		Description: "View for SiteExist errors",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{tagCertName},
		Measure:     mSiteExistsErrors,
	},
	{
		Name:        "caddytls/site_load_errors",
		Description: "View for LoadSite errors",
		Aggregation: view.Count(),
		Measure:     mSiteLoadErrors,
		TagKeys:     []tag.Key{tagCertName},
	},
	{
		Name:        "caddytls/site_not_exists_errors",
		Description: "View for the number of non-existent site counts",
		Aggregation: view.Count(),
		Measure:     mSiteNotExistsErrors,
		TagKeys:     []tag.Key{tagCertName},
	},
	{
		Name:        "caddytls/failed_certfile_deletions",
		Description: "View for the number of failed certificate file deletions",
		Aggregation: view.Count(),
		Measure:     mFailedCertFileDeletions,
		TagKeys:     []tag.Key{tagCertName},
	},
	{
		Name:        "caddytls/obtain_invocations",
		Description: "View for the number of Obtain invocations",
		Aggregation: view.Count(),
		Measure:     mObtainCalls,
		TagKeys:     []tag.Key{tagCertName},
	},
	{
		Name:        "caddytls/obtain_failures",
		Description: "View for the number of failed Obtain invocations",
		Aggregation: view.Count(),
		Measure:     mObtainFailures,
		TagKeys:     []tag.Key{tagCertName},
	},
	{
		Name:        "caddytls/obtain_successes",
		Description: "View for the number of successful Obtain invocations",
		Aggregation: view.Count(),
		Measure:     mObtainSuccesses,
		TagKeys:     []tag.Key{tagCertName},
	},
	{
		Name:        "caddytls/obtain_blank_certs",
		Description: "View for the number of blank Obtain certificates",
		Aggregation: view.Count(),
		Measure:     mObtainBlankCertificates,
		TagKeys:     []tag.Key{tagCertName},
	},
	{
		Name:        "caddytls/save_cert_errors",
		Description: "View for the number of failed save certificate errors",
		Aggregation: view.Count(),
		Measure:     mSaveCertErrors,
		TagKeys:     []tag.Key{tagCertName},
	},
	{
		Name:        "caddytls/renew_invocations",
		Description: "View for the number of Renew invocations",
		Aggregation: view.Count(),
		Measure:     mRenewCalls,
		TagKeys:     []tag.Key{tagCertName},
	},
	{
		Name:        "caddytls/renew_failures",
		Description: "View for the number of failed Renew invocations",
		Aggregation: view.Count(),
		Measure:     mRenewFailures,
		TagKeys:     []tag.Key{tagCertName},
	},
	{
		Name:        "caddytls/renew_successes",
		Description: "View for the number of successful Renew invocations",
		Aggregation: view.Count(),
		Measure:     mRenewSuccesses,
		TagKeys:     []tag.Key{tagCertName},
	},
}

func enableDistributedMonitoring() error {
	return view.Register(Views...)
}
