// Package usecases orchestrates application behavior on top of domain values
// and ports. Each use case is a function that receives the ports it needs,
// delegates pure decisions to the domain package, and returns domain values
// to its caller. Use cases never import infrastructure or CLI code.
package usecases
