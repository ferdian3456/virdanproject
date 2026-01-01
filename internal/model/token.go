package model

type TokenResponse struct {
	AccessToken           string `json:"accessToken"`
	AccessTokenExpiresIn  int    `json:"accessTokenExpiresIn"`
	RefreshToken          string `json:"refreshToken"`
	RefreshTokenExpiresIn int    `json:"refreshTokenExpiresIn"`
	TokenType             string `json:"tokenType"`
}
