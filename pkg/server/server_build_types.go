package server

import (
	driverspkg "github.com/relaymesh/githook/pkg/drivers"
	"github.com/relaymesh/githook/pkg/providerinstance"
	driversstore "github.com/relaymesh/githook/pkg/storage/drivers"
	"github.com/relaymesh/githook/pkg/storage/eventlogs"
	"github.com/relaymesh/githook/pkg/storage/installations"
	"github.com/relaymesh/githook/pkg/storage/namespaces"
	providerinstancestore "github.com/relaymesh/githook/pkg/storage/provider_instances"
	"github.com/relaymesh/githook/pkg/storage/rules"
)

type serverStores struct {
	installStore   *installations.Store
	namespaceStore *namespaces.Store
	ruleStore      *rules.Store
	logStore       *eventlogs.Store
	driverStore    *driversstore.Store
	instanceStore  *providerinstancestore.Store
}

type serverCaches struct {
	driverCache        *driverspkg.Cache
	instanceCache      *providerinstance.Cache
	dynamicDriverCache *driverspkg.DynamicPublisherCache
}
