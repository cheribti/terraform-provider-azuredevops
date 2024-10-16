package identity

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/microsoft/terraform-provider-azuredevops/azuredevops/internal/client"
)

// DataIdentityUserResource returns the user data source resource
func DataIdentityUser() *schema.Resource {
	return &schema.Resource{
		Read: dataIdentitySourceUserRead,
		Timeouts: &schema.ResourceTimeout{
			Read: schema.DefaultTimeout(5 * time.Minute),
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
			},
			"descriptor": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"search_filter": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "General",
				ValidateFunc: validation.StringInSlice([]string{"AccountName", "DisplayName", "MailAddress", "General"}, false),
			},
		},
	}
}

// Lecture de la ressource de données.
func dataIdentitySourceUserRead(d *schema.ResourceData, m interface{}) error {
	clients := m.(*client.AggregatedClient)
	userName := d.Get("name").(string)
	searchFilter := d.Get("search_filter").(string)

	// Query ADO for list of identity user with filter
	filterUser, err := getIdentityUsersWithFilterValue(clients, searchFilter, userName)
	if err != nil {
		return fmt.Errorf(" Finding user with filter %s. Error: %v", searchFilter, err)
	}

	flattenUser, err := flattenIdentityUsers(filterUser)
	if err != nil {
		return fmt.Errorf(" Fatten user. Error: %v", err)
	}

	// Filter for the desired user in the FilterUsers results
	targetUser := validateIdentityUser(flattenUser, userName, searchFilter)
	if targetUser == nil {
		return fmt.Errorf(" Could not find user with name: %s with filter: %s", userName, searchFilter)
	}

	// Set id and user list for users data resource
	targetUserID := targetUser.Id.String()
	fmt.Printf("Setting user ID: %s\n", targetUserID)                          // Log the user ID being set
	fmt.Printf("User Subject Descriptor: %s\n", *targetUser.SubjectDescriptor) // Log the subject descriptor
	d.SetId(targetUserID)
	d.Set("descriptor", targetUser.SubjectDescriptor)
	return nil
}

//	interroge ADO pour récupérer une liste des users en fonction du filtre
//
// Query AZDO for users with matching filter and search string
func getIdentityUsersWithFilterValue(clients *client.AggregatedClient, searchFilter string, filterValue string) (*[]identity.Identity, error) {
	// Get list of user with search filter and filter value provided at data source invocation.
	response, err := clients.IdentityClient.ReadIdentities(clients.Ctx, identity.ReadIdentitiesArgs{
		SearchFilter: &searchFilter, // Filter to get users
		FilterValue:  &filterValue,  // Search String for user
	})

	if err != nil {
		return nil, err
	}
	return response, nil
}

// Flatten Query Results
// transforme la liste des users récupérée en un format simplifié (vérifie que chaque utilisateur possède un descriptor valide avant de l'ajouter à la liste transformée).
func flattenIdentityUsers(users *[]identity.Identity) (*[]identity.Identity, error) {
	if users == nil {
		return nil, fmt.Errorf(" Input Users Parameter is nil")
	}
	results := make([]identity.Identity, len(*users))
	for i, user := range *users {
		if user.SubjectDescriptor == nil {
			return nil, fmt.Errorf(" User Object does not contain an id")
		} else {
			fmt.Printf("User %v has SubjectDescriptor: %v\n", user.ProviderDisplayName, *user.SubjectDescriptor)
		}
		newUser := identity.Identity{
			Descriptor:          user.SubjectDescriptor,
			Id:                  user.Id,
			ProviderDisplayName: user.ProviderDisplayName,
			// Add other fields here if needed
		}

		// afficher le descriptor
		fmt.Printf("Descriptor for the new user: %s\n", *newUser.Descriptor)
		results[i] = newUser
	}
	return &results, nil
}

//filtre les utilisateurs en cherchant ceux dont le nom d'affichage contient la chaîne de caractères fournie (userName), en utilisant une comparaison insensible à la casse.

// Filter results to validate user is correct. Occurs post-flatten due to missing properties based on search-filter.
func validateIdentityUser(users *[]identity.Identity, userName string, searchFilter string) *identity.Identity {
	for _, user := range *users {
		if strings.Contains(strings.ToLower(*user.ProviderDisplayName), strings.ToLower(userName)) {
			return &user
		}
	}
	return nil
}
