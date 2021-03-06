package eth

import (
	"encoding/xml"
	"fmt"

	"github.com/PaloAltoNetworks/pango/namespace"
	"github.com/PaloAltoNetworks/pango/util"
	"github.com/PaloAltoNetworks/pango/version"
)

// PanoEth is the client.Network.EthernetInterface namespace.
type PanoEth struct {
	con util.XapiClient
	ns  *namespace.Namespace
}

// Initialize is invoked by client.Initialize().
func (c *PanoEth) Initialize(con util.XapiClient) {
	c.con = con
	c.ns = namespace.New(singular, plural, con)
}

// ShowList performs SHOW to retrieve a list of ethernet interfaces.
func (c *PanoEth) ShowList(tmpl, ts string) ([]string, error) {
	result, _ := c.versioning()
	return c.ns.Listing(util.Show, c.xpath(tmpl, ts, nil), result)
}

// GetList performs GET to retrieve a list of ethernet interfaces.
func (c *PanoEth) GetList(tmpl, ts string) ([]string, error) {
	result, _ := c.versioning()
	return c.ns.Listing(util.Get, c.xpath(tmpl, ts, nil), result)
}

// Get performs GET to retrieve information for the given ethernet interface.
func (c *PanoEth) Get(tmpl, ts, name string) (Entry, error) {
	result, _ := c.versioning()
	if err := c.ns.Object(util.Get, c.xpath(tmpl, ts, []string{name}), name, result); err != nil {
		return Entry{}, err
	}

	return result.Normalize()[0], nil
}

// GetAll performs GET to retrieve information for all objects.
func (c *PanoEth) GetAll(tmpl, ts string) ([]Entry, error) {
	result, _ := c.versioning()
	if err := c.ns.Objects(util.Get, c.xpath(tmpl, ts, nil), result); err != nil {
		return nil, err
	}

	return result.Normalize(), nil
}

// Show performs SHOW to retrieve information for the given ethernet interface.
func (c *PanoEth) Show(tmpl, ts, name string) (Entry, error) {
	result, _ := c.versioning()
	if err := c.ns.Object(util.Show, c.xpath(tmpl, ts, []string{name}), name, result); err != nil {
		return Entry{}, err
	}

	return result.Normalize()[0], nil
}

// ShowAll performs SHOW to retrieve information for all objects.
func (c *PanoEth) ShowAll(tmpl, ts string) ([]Entry, error) {
	result, _ := c.versioning()
	if err := c.ns.Objects(util.Show, c.xpath(tmpl, ts, nil), result); err != nil {
		return nil, err
	}

	return result.Normalize(), nil
}

// Set performs SET to create / update one or more ethernet interfaces.
//
// Specifying a non-empty vsys will import the interfaces into that vsys,
// allowing the vsys to use them, as long as the interface does not have a
// mode of "ha" or "aggregate-group".  Interfaces that have either of those
// modes are omitted from this function's followup vsys import.
func (c *PanoEth) Set(tmpl, ts, vsys string, e ...Entry) error {
	var err error

	if len(e) == 0 {
		return nil
	} else if tmpl == "" && ts == "" {
		return fmt.Errorf("tmpl or ts must be specified")
	} else if vsys == "" {
		return fmt.Errorf("vsys must be specified, not %q", vsys)
	}

	_, fn := c.versioning()
	n1 := make([]string, len(e))
	n2 := make([]string, 0, len(e))

	// Build up the struct with the given interface configs.
	d := util.BulkElement{XMLName: xml.Name{Local: "ethernet"}}
	for i := range e {
		d.Data = append(d.Data, fn(e[i]))
		n1[i] = e[i].Name
		if e[i].Mode != "ha" && e[i].Mode != "aggregate-group" {
			n2 = append(n2, e[i].Name)
		}
	}
	c.con.LogAction("(set) %s: %v", plural, n1)

	// Set xpath.
	path := c.xpath(tmpl, ts, n1)
	if len(e) == 1 {
		path = path[:len(path)-1]
	} else {
		path = path[:len(path)-2]
	}

	// Create the interfaces.
	_, err = c.con.Set(path, d.Config(), nil, nil)
	if err != nil {
		return err
	}

	// Remove the interfaces from any vsys they're currently in.
	if err = c.con.VsysUnimport(util.InterfaceImport, tmpl, ts, n1); err != nil {
		return err
	}

	// Perform vsys import next.
	return c.con.VsysImport(util.InterfaceImport, tmpl, ts, vsys, n2)
}

// Edit performs EDIT to create / update the specified ethernet interface.
//
// Specifying a non-empty vsys will import the interface into that vsys,
// allowing the vsys to use it, as long as the interface does not have a
// mode of "ha" or "aggregate-group".  Interfaces that have either of those
// modes are omitted from this function's followup vsys import.
func (c *PanoEth) Edit(tmpl, ts, vsys string, e Entry) error {
	var err error

	if tmpl == "" && ts == "" {
		return fmt.Errorf("tmpl or ts must be specified")
	} else if vsys == "" {
		return fmt.Errorf("vsys must be specified")
	}

	_, fn := c.versioning()

	c.con.LogAction("(edit) %s: %q", singular, e.Name)

	// Set xpath.
	path := c.xpath(tmpl, ts, []string{e.Name})

	// Edit the interface.
	_, err = c.con.Edit(path, fn(e), nil, nil)
	if err != nil {
		return err
	}

	// Remove the interface from any vsys it's currently in.
	if err = c.con.VsysUnimport(util.InterfaceImport, tmpl, ts, []string{e.Name}); err != nil {
		return err
	}

	// Check if we should skip the import step.
	if e.Mode == "ha" || e.Mode == "aggregate-group" {
		return nil
	}

	// Import the interface.
	return c.con.VsysImport(util.InterfaceImport, tmpl, ts, vsys, []string{e.Name})
}

// Delete removes the given interface(s) from the firewall.
//
// Interfaces can be a string or an Entry object.
func (c *PanoEth) Delete(tmpl, ts string, e ...interface{}) error {
	var err error

	if len(e) == 0 {
		return nil
	} else if tmpl == "" && ts == "" {
		return fmt.Errorf("tmpl or ts must be specified")
	}

	names := make([]string, len(e))
	for i := range e {
		switch v := e[i].(type) {
		case string:
			names[i] = v
		case Entry:
			names[i] = v.Name
		default:
			return fmt.Errorf("Unknown type sent to delete: %s", v)
		}
	}
	c.con.LogAction("(delete) %s: %v", plural, names)

	// Unimport interfaces.
	if err = c.con.VsysUnimport(util.InterfaceImport, tmpl, ts, names); err != nil {
		return err
	}

	// Remove interfaces next.
	path := c.xpath(tmpl, ts, names)
	_, err = c.con.Delete(path, nil, nil)
	return err
}

/** Internal functions for this namespace struct **/

func (c *PanoEth) versioning() (normalizer, func(Entry) interface{}) {
	v := c.con.Versioning()

	if v.Gte(version.Number{9, 0, 0, ""}) {
		return &container_v4{}, specify_v4
	} else if v.Gte(version.Number{8, 1, 0, ""}) {
		return &container_v3{}, specify_v3
	} else if v.Gte(version.Number{7, 1, 0, ""}) {
		return &container_v2{}, specify_v2
	} else {
		return &container_v1{}, specify_v1
	}
}

func (c *PanoEth) xpath(tmpl, ts string, vals []string) []string {
	ans := make([]string, 0, 12)
	ans = append(ans, util.TemplateXpathPrefix(tmpl, ts)...)
	ans = append(ans,
		"config",
		"devices",
		util.AsEntryXpath([]string{"localhost.localdomain"}),
		"network",
		"interface",
		"ethernet",
		util.AsEntryXpath(vals),
	)

	return ans
}
