package main

// return true as soon as we find an item in set that is also in found
func UserInGroup(set []string, found[]string) bool {
	for _, item := range set {
		for _, foundGroup := range found {
			if item == foundGroup {
				return true
			}
		}
	}
return false
}
